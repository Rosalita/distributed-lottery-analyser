package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/data"
)

func main() {
	fmt.Println("Distributed Lottery Analyzer is starting up...")

	// The distributed-compute-operator should assign predictable hostnames.
	// For example: "lottery-job-leader" or "lottery-job-worker-0"
	hostname, _ := os.Hostname()

	// Default to leader if running locally (not inside Kubernetes)
	isLocal := os.Getenv("KUBERNETES_SERVICE_HOST") == ""
	isLeader := strings.HasSuffix(hostname, "-leader") || hostname == "" || isLocal

	if isLeader {
		fmt.Println("Role: Leader. Initializing data manager...")

		// Navigate locally to the getdrawhistory/data folder
		_, currentFile, _, _ := runtime.Caller(0)
		baseDataDir := filepath.Join(filepath.Dir(currentFile), "..", "getdrawhistory", "data")

		allGameData, err := data.LoadAllData(baseDataDir)
		if err != nil {
			fmt.Printf("Failed to load game data: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully loaded historical data for %d games.\n", len(allGameData))

		// Look up Thunderball draw 3923 directly
		if draw, exists := allGameData["thunderball"][3923]; exists {
			fmt.Println("***")
			fmt.Printf("Found Draw 3923: %+v\n", draw)
			fmt.Println("***")
		} else {
			fmt.Println("Draw 3923 not found in the dataset.")
		}
	} else {
		fmt.Println("Role: Worker. Waiting for tasks from the leader...")
	}

	// Block indefinitely to prevent the container from immediately exiting.
	// We use a sleep loop instead of select{} to avoid Go deadlock panics
	// before we have implemented our background servers and goroutines.
	for {
		time.Sleep(1 * time.Hour)
	}
}
