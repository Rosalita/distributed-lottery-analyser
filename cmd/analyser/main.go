package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println("Distributed Lottery Analyzer is starting up...")

	// The distributed-compute-operator should assign predictable hostnames.
	// For example: "lottery-job-leader" or "lottery-job-worker-0"
	hostname, _ := os.Hostname()

	if strings.HasSuffix(hostname, "-leader") || hostname == "" {
		fmt.Println("Role: Leader. Initializing data manager...")
		// TODO: Load the master CSV files from the local disk into memory instead of making API calls
	} else {
		fmt.Println("Role: Worker. Waiting for tasks from the leader...")
	}

	// Block indefinitely to prevent the container from immediately exiting
	select {}
}
