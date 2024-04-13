package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Set up a channel to catch SIGINT (Ctrl+C)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT)

	// Run tasks until interrupted.
	ticker := time.NewTicker(1 * time.Second)
	fmt.Println("comprehend.dev agent started")
	for {
		select {
		case <-interrupt:
			fmt.Println("comprehend.dev agent exiting")
			return
		case <-ticker.C:
			// Do tasks
		}
	}
}
