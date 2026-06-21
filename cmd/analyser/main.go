package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/coordinator"
	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/data"
	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/evaluator"
	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/worker"
	analyserPb "github.com/Rosalita/distributed-lottery-analyser/protos/generated/analyser"

	"google.golang.org/grpc"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("Distributed Lottery Analyzer is starting up...")

	roleStr := flag.String("role", "", "Role: leader or worker (defaults to auto-detect)")
	portStr := flag.String("port", "50051", "Leader port to run gRPC server on")
	leaderAddr := flag.String("leader", "localhost:50051", "Leader address for worker to connect to")
	gameStr := flag.String("game", "thunderball", "Game name to analyze: lotto, euromillions, thunderball, setforlife")
	chunkSize := flag.Int64("chunk-size", 100_000, "Solver combinations chunk size distributed to workers")
	limit := flag.Int("limit", 5, "Number of top-performing tickets to keep")

	flag.Parse()

	// Determine role
	role := *roleStr
	if role == "" {
		hostname, _ := os.Hostname()
		isLocal := os.Getenv("KUBERNETES_SERVICE_HOST") == ""
		if strings.HasSuffix(hostname, "-leader") || hostname == "" || isLocal {
			role = "leader"
		} else {
			role = "worker"
		}
	}

	if role == "leader" {
		log.Printf("Role: Leader. Game: %s. Chunk size: %d. Top tickets limit: %d", *gameStr, *chunkSize, *limit)

		config, exists := evaluator.GetGameConfig(*gameStr)
		if !exists {
			log.Fatalf("Invalid game name: %s", *gameStr)
		}

		// Navigate locally to the getdrawhistory/data folder
		_, currentFile, _, _ := runtime.Caller(0)
		baseDataDir := filepath.Join(filepath.Dir(currentFile), "..", "getdrawhistory", "data")

		allGameData, err := data.LoadAllData(baseDataDir)
		if err != nil {
			log.Fatalf("Failed to load historical game data: %v", err)
		}

		gameData, exists := allGameData[config.Name]
		if !exists || len(gameData) == 0 {
			log.Fatalf("No historical draw details found for game: %s", config.Name)
		}
		log.Printf("Successfully loaded %d historical draw details for %s.", len(gameData), config.Name)

		// Instantiate Coordinator (timeout: 30 seconds for chunks)
		coord := coordinator.NewCoordinator(config, gameData, *chunkSize, *limit, 30*time.Second)

		// Start gRPC server
		lis, err := net.Listen("tcp", ":"+*portStr)
		if err != nil {
			log.Fatalf("Failed to listen on port %s: %v", *portStr, err)
		}

		grpcServer := grpc.NewServer()
		analyserPb.RegisterAnalyserServer(grpcServer, coord)

		go func() {
			log.Printf("Leader gRPC server listening on port %s...", *portStr)
			if err := grpcServer.Serve(lis); err != nil {
				log.Fatalf("gRPC server failed: %v", err)
			}
		}()

		// Print periodic progress
		go func() {
			for {
				time.Sleep(5 * time.Second)
				comp, total := coord.Progress()
				percentage := 0.0
				if total > 0 {
					percentage = float64(comp) / float64(total) * 100.0
				}
				log.Printf("[Progress] %d of %d chunks completed (%.2f%%)", comp, total, percentage)
				if coord.AllDone() {
					break
				}
			}
		}()

		// Block until completion
		for {
			time.Sleep(1 * time.Second)
			if coord.AllDone() {
				log.Println("All chunks completed successfully!")
				grpcServer.GracefulStop()
				break
			}
		}

		// Print final results
		tickets := coord.GetTopTickets()
		fmt.Println("\n==================================================")
		fmt.Printf("TOP %d MOST PROFITABLE TICKETS FOR %s\n", len(tickets), strings.ToUpper(config.Name))
		fmt.Println("==================================================")
		for idx, t := range tickets {
			fmt.Printf("%d. Primary: %v, Secondary: %v | Total Earnings: £%s\n",
				idx+1, t.PrimaryNumbers, t.SecondaryNumbers, formatPence(t.TotalPrizePence))
		}
		fmt.Println("==================================================")

	} else {
		log.Printf("Role: Worker. Connecting to leader at %s...", *leaderAddr)

		err := worker.RunWorker(*leaderAddr, *limit)
		if err != nil {
			log.Fatalf("Worker client encountered error: %v", err)
		}

		log.Println("Worker completed all work assignments. Exiting.")
	}
}

func formatPence(pence int64) string {
	pounds := pence / 100
	remainder := pence % 100

	poundsStr := strconv.FormatInt(pounds, 10)
	var formattedPounds []byte

	n := len(poundsStr)
	for i := 0; i < n; i++ {
		if i > 0 && (n-i)%3 == 0 {
			formattedPounds = append(formattedPounds, ',')
		}
		formattedPounds = append(formattedPounds, poundsStr[i])
	}

	return fmt.Sprintf("%s.%02d", string(formattedPounds), remainder)
}
