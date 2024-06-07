package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"github.com/comprehend-dev/comprehend.dev/agent/collectors"
	"github.com/go-ini/ini"
)

func main() {
	// Set up a channel to catch SIGINT (Ctrl+C)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT)

	activeCollectors := []collectors.Collector{};

	flag.Func("config", "Path to configuration file", func(path string) error {
		options := ini.LoadOptions{}
		options.AllowNonUniqueSections = true
		file, err := ini.LoadSources(options, path)
		if err != nil {
			log.Fatalf("Error loading configuration file: %s", err)
		}
		for _, section := range file.Sections() {
			if section.Name() == "DEFAULT" {
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
		activeCollector, err := collectors.Collectors[schema].Initialize(arg)
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

	// Run tasks until interrupted.
	ticker := time.NewTicker(60 * time.Second)
	fmt.Println("comprehend.dev agent started")

	for _, collector := range activeCollectors {
		model, err := collector.Collect()
		if err != nil {
			log.Fatalf("Error trying to collect: %s", err)
		}
		log.Println(model)
	}

	for {
		select {
		case <-interrupt:
			fmt.Println("comprehend.dev agent exiting")
			return
		case <-ticker.C:
			for _, collector := range activeCollectors {
				model, err := collector.Collect()
				if err != nil {
					log.Fatalf("Error trying to collect: %s", err)
				}
				log.Println(model)
			}
		}
	}
}
