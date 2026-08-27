package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"net/url"
	"github.com/comprehend-dev/collector/collectors"
	"github.com/go-ini/ini"
)

const defaultComprehendURL = "https://ingestion.comprehend.dev/"

// Replaced at build time with the released version; see VERSION in the Makefile.
var version = "dev"

func main() {
	// Set up a channel to catch SIGINT (Ctrl+C)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT)

	activeCollectors := []collectors.Collector{}

	var apiKey string
	var organization string
	var comprehendURL string

	flag.Func("config", "Path to configuration file", func(path string) error {
		options := ini.LoadOptions{}
		options.AllowNonUniqueSections = true
		file, err := ini.LoadSources(options, path)
		if err != nil {
			log.Fatalf("Error loading configuration file: %s", err)
		}
		for _, section := range file.Sections() {
			if section.Name() == "DEFAULT" {
				if apiKey == "" {
					apiKey = section.Key("apikey").String();
				}
				if organization == "" {
					organization = section.Key("organization").String();
				}
				if comprehendURL == "" || comprehendURL == defaultComprehendURL {
					comprehendURL = section.Key("comprehend-url").String();
				}
				continue
			}
			collector, found := collectors.Collectors[section.Name()]
			if !found {
				log.Fatalf("Error in configuration: unknown collector %s", section.Name())
			}
			activeCollector, err := collector.InitializeFromConfig(section)
			if err != nil {
				log.Fatalf("Error in configuration %s: %s", section.Name(), err)
			}
			activeCollectors = append(activeCollectors, activeCollector)
		}
		return nil
	})

	flag.StringVar(&apiKey, "apikey", apiKey, "The comprehend.dev API key you created (\"API Keys\" on the SDKs page)");
	flag.StringVar(&organization, "organization", organization, "Your comprehend.dev organization slug.");
	flag.StringVar(&comprehendURL, "comprehend-url", defaultComprehendURL, "The URL of the ingestion service - for development use only.");
	showVersion := flag.Bool("version", false, "Print the collector version and exit.");

	for _, name := range collectors.Names() {
		collector := collectors.Collectors[name]
		flag.Func(name, collector.Description(), func(arg string) error {
			activeCollector, err := collector.Initialize(arg);
			if err != nil {
				log.Fatalf("Error in argument \"%s\": %s", arg, err)
			}
			activeCollectors = append(activeCollectors, activeCollector)
			return nil
		})
	}

	flag.Usage = func() {
		// Descriptions line up in a column, so that the widest option still leaves two spaces.
		entry := func(option string, description string) {
			fmt.Fprintf(flag.CommandLine.Output(), "    %-22s  %s\n", option, description)
		}
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] [arguments]\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		entry("--config <path>", "A configuration file containing additional options.")
		entry("--apikey <value>", "The comprehend.dev API key you created (\"API Keys\" on the SDKs page).")
		entry("--organization <value>", "Your comprehend.dev organization slug.")
		entry("--version", "Print the collector version and exit.")
		// comprehend-url is intentionally undocumented as it's meant for development use only
		for _, name := range collectors.Names() {
			entry(fmt.Sprintf("--%s <value>", name), collectors.Collectors[name].Description())
		}
		fmt.Fprintf(flag.CommandLine.Output(), "Arguments:\n")
		for _, name := range collectors.Names() {
			entry(fmt.Sprintf("%s://...", name), collectors.Collectors[name].Description())
		}
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("comprehend.dev collector %s\n", version)
		return
	}

	for idx, arg := range flag.Args() {
		schema, _, found := strings.Cut(arg, ":")
		if found == false {
			log.Fatalf("Argument \"%s\" does not look like a URI", arg)
		}
		collector, found := collectors.Collectors[schema]
		if !found {
			log.Fatalf("Error in argument %d (\"%s\"): unknown collector %s", idx + 1, arg, schema)
		}
		activeCollector, err := collector.Initialize(arg)
		if err != nil {
			log.Fatalf("Error in positional argument %d (\"%s\"): %s", idx + 1, arg, err)
		}
		activeCollectors = append(activeCollectors, activeCollector)
	}

	if len(activeCollectors) == 0 {
		// No arguments. Give collectors a chance to auto-detect
		var errors string = ""
		for _, name := range collectors.Names() {
			collector := collectors.Collectors[name]
			defaultCollector, err := collector.InitializeDefault()
			if err != nil {
				errors = fmt.Sprintf("%s%s: %s\n", errors, collector.URISchema(), err)
			} else if defaultCollector != nil {
				activeCollectors = append(activeCollectors, defaultCollector);
			}
		}

		if len(activeCollectors) == 0 {
			log.Fatalf("Did not find anything to collect. Please specify things you want to sync via command line options.\nSee --help for details.\n\nErrors encountered in auto-detection:\n%s", errors)
		} else {
			names := activeCollectors[0].URISchema()
			for _, collector := range activeCollectors[1:] {
				names = fmt.Sprintf("%s, %s", names, collector.URISchema())
			}
			log.Printf("Found these collectors automatically: %s\n", names)
		}
	}

	client := &http.Client{}
	comprehendURL = comprehendURL + url.PathEscape(organization) + "/sync/"

	collect := func () {
		for _, collector := range activeCollectors {
			reqURL := comprehendURL + collector.URISchema()

			model, err := collector.Collect()
			if err != nil {
				log.Fatalf("Error trying to collect: %s", err)
			}

			json, err := model.ToJSON()
			if err != nil {
				log.Fatalf("Error getting JSON from model: %s", err)
			}

			req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewBuffer(json))
			if err != nil {
				log.Fatalf("Could not create http request: %s\n", err)
			}

			req.Header.Add("Authorization", "Bearer " + apiKey)
			req.Header.Add("Content-Type", "application/json")
			res, err := client.Do(req)
			if err != nil {
				log.Fatalf("Error making http request: %s\n", err)
			}

			if res.StatusCode != 204 {
				var body []byte
				res.Body.Read(body)
				log.Fatalf("Got unexpected response from server: %s\n\n%s\n", res.Status, body)
			}
			res.Body.Close()
		}
	}

	// Run tasks until interrupted.
	ticker := time.NewTicker(60 * time.Second)
	log.Println("comprehend.dev collector started for organization", organization)

	// Collect once immediately before we go into the waiting loop
	collect()

	for {
		select {
		case <-interrupt:
			log.Println("comprehend.dev collector exiting")
			return
		case <-ticker.C:
			collect()
		}
	}
}
