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
	"github.com/comprehend-dev/comprehend.dev/agent/collectors"
	"github.com/go-ini/ini"
)

func main() {
	// Set up a channel to catch SIGINT (Ctrl+C)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT)

	activeCollectors := []collectors.Collector{}

	var apiKey string
	var organization string
	var document string
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
				if document == "" {
					document = section.Key("document").String();
				}
				if comprehendURL == "" {
					document = section.Key("comprehend_url").String();
				}
				continue
			}
			collector, found := collectors.Collectors[section.Name()]
			if !found {
				log.Fatalf("Error in configuration: unknown collector %s", section.Name())
			}
			activeCollector, err := collector.InitializeFromConfig(section.Keys())
			if err != nil {
				log.Fatalf("Error in configuration %s: %s", section.Name(), err)
			}
			activeCollectors = append(activeCollectors, activeCollector)
		}
		return nil
	})

	flag.StringVar(&apiKey, "apikey", apiKey, "The comprehend.dev API key you created (\"API Keys\" in the menu)");
	flag.StringVar(&organization, "organization", organization, "Your comprehend.dev organization slug.");
	flag.StringVar(&document, "document", organization, "The document to import the schema to.");
	flag.StringVar(&comprehendURL, "comprehend-url", "https://ingest.comprehend.dev/", "The document to import the schema to.");

	for name, collector := range collectors.Collectors {
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
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] [arguments]\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    --config <path>     A configuration file containing additional options.\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    --apikey <value>    The comprehend.dev API key you created (\"API Keys\" in the menu).\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    --organization <value>    Your comprehend.dev organization slug.\n")
		fmt.Fprintf(flag.CommandLine.Output(), "    --document <value>    The document to import the schema to.\n")
		// comprehend-url is intentionally undocumented as it's meant for development use only
		for name, collector := range collectors.Collectors {
			fmt.Fprintf(flag.CommandLine.Output(), "    --%s <value>    %s\n", name, collector.Description())
		}
		fmt.Fprintf(flag.CommandLine.Output(), "Arguments:\n")
		for name, collector := range collectors.Collectors {
			fmt.Fprintf(flag.CommandLine.Output(), "    %s://...    %s\n", name, collector.Description())
		}
	}

	flag.Parse()

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
		for _, collector := range collectors.Collectors {
			defaultCollector, err := collector.InitializeDefault()
			if err != nil {
				log.Fatalf("Error trying to auto-detect things to collect: %s\n\nYou may need to specify things to collect manually using command line options.\nSee --help for details.", err)
			}
			if defaultCollector != nil {
				activeCollectors = append(activeCollectors, defaultCollector);
			}
		}

		if len(activeCollectors) == 0 {
			log.Fatal("Did not find anything to collect. Please specify things you want to sync via command line options.\nSee --help for details.")
		}
	}

	client := &http.Client{}
	comprehendURL = comprehendURL + url.PathEscape(organization) + "/sync/"

	collect := func () {
		for _, collector := range activeCollectors {
			url := comprehendURL + collector.URISchema() + "?document=" + url.QueryEscape(document)

			model, err := collector.Collect()
			if err != nil {
				log.Fatalf("Error trying to collect: %s", err)
			}

			json, err := model.ToJSON()
			if err != nil {
				log.Fatalf("Error getting JSON from model: %s", err)
			}

			req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(json))
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
	fmt.Println("comprehend.dev agent started for organization ", organization, ", importing into document ", document)

	// Collect once immediately before we go into the waiting loop
	collect()

	for {
		select {
		case <-interrupt:
			fmt.Println("comprehend.dev agent exiting")
			return
		case <-ticker.C:
			collect()
		}
	}
}
