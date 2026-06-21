package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/data"
	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/evaluator"
	analyserPb "github.com/Rosalita/distributed-lottery-analyser/protos/generated/analyser"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RunWorker connects to the Leader gRPC server and runs the worker processing loop.
func RunWorker(leaderAddr string, limit int) error {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "worker"
	}
	workerID := fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano())

	log.Printf("Starting worker %s...", workerID)

	// Connect to leader gRPC server
	conn, err := grpc.NewClient(leaderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to dial leader at %s: %w", leaderAddr, err)
	}
	defer conn.Close()

	client := analyserPb.NewAnalyserClient(conn)
	ctx := context.Background()

	log.Printf("Registering worker with leader...")
	regResp, err := client.RegisterWorker(ctx, &analyserPb.RegisterWorkerRequest{WorkerId: workerID})
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	pbConfig := regResp.GetGameConfig()
	log.Printf("Successfully registered worker. Game: %s, Combinations: %d", pbConfig.GetName(), regResp.GetTotalCombinations())

	// Map pb game configuration
	config := evaluator.GameConfig{
		Name:            pbConfig.GetName(),
		PrimaryCount:    int(pbConfig.GetPrimaryCount()),
		PrimarySelect:   int(pbConfig.GetPrimarySelect()),
		SecondaryCount:  int(pbConfig.GetSecondaryCount()),
		SecondarySelect: int(pbConfig.GetSecondarySelect()),
	}

	// Convert pb draws to optimized FastDraw representation
	pbDraws := regResp.GetDraws()
	fastDraws := make([]evaluator.FastDraw, len(pbDraws))
	for i, pbDraw := range pbDraws {
		fastDraws[i] = evaluator.NewFastDraw(mapDraw(pbDraw), config.Name)
	}
	log.Printf("Pre-parsed %d historical draws into evaluator masks.", len(fastDraws))

	// Solver Loop
	for {
		workResp, err := client.GetWork(ctx, &analyserPb.GetWorkRequest{WorkerId: workerID})
		if err != nil {
			return fmt.Errorf("failed to fetch next work chunk: %w", err)
		}

		if workResp.GetNoMoreWork() {
			log.Println("Received termination signal (no more work). Exiting gracefully.")
			break
		}

		startRank := workResp.GetStartRank()
		endRank := workResp.GetEndRank()
		log.Printf("Processing chunk [%d, %d)...", startRank, endRank)

		startEval := time.Now()
		topTickets := evaluator.EvaluateRange(startRank, endRank, config, fastDraws, limit)
		duration := time.Since(startEval)

		rate := float64(endRank-startRank) / duration.Seconds()
		log.Printf("Chunk [%d, %d) completed in %v (%.2f combinations/sec)", startRank, endRank, duration, rate)

		// Convert evaluator.Ticket to analyserPb.Ticket
		pbTickets := make([]*analyserPb.Ticket, len(topTickets))
		for i, t := range topTickets {
			pbTickets[i] = &analyserPb.Ticket{
				PrimaryNumbers:   intToInt32Slice(t.PrimaryNumbers),
				SecondaryNumbers: intToInt32Slice(t.SecondaryNumbers),
				TotalPrizePence:  t.TotalPrizePence,
			}
		}

		// Report results back to leader
		reportResp, err := client.ReportResult(ctx, &analyserPb.ReportResultRequest{
			WorkerId:   workerID,
			StartRank:  startRank,
			EndRank:    endRank,
			TopTickets: pbTickets,
		})
		if err != nil {
			log.Printf("Warning: failed to report result: %v", err)
			continue
		}
		if !reportResp.GetSuccess() {
			log.Printf("Warning: leader rejected report: %s", reportResp.GetMessage())
		}
	}

	return nil
}

func mapDraw(pbDraw *analyserPb.DrawDetails) data.DrawDetails {
	t, _ := time.Parse(time.RFC3339, pbDraw.GetDrawResult().GetDrawDate())

	primarySlice := make([]int, len(pbDraw.GetDrawResult().GetDrawnNumbers().GetPrimaryNumbers()))
	for i, v := range pbDraw.GetDrawResult().GetDrawnNumbers().GetPrimaryNumbers() {
		primarySlice[i] = int(v)
	}

	secondarySlice := make([]int, len(pbDraw.GetDrawResult().GetDrawnNumbers().GetSecondaryNumbers()))
	for i, v := range pbDraw.GetDrawResult().GetDrawnNumbers().GetSecondaryNumbers() {
		secondarySlice[i] = int(v)
	}

	levels := make([]data.PrizeLevel, len(pbDraw.GetPrizeBreakdown().GetPrizeLevels()))
	for i, l := range pbDraw.GetPrizeBreakdown().GetPrizeLevels() {
		levels[i] = data.PrizeLevel{
			DrawRound:          l.GetDrawRound(),
			MatchLabel:         l.GetMatchLabel(),
			MatchBallPrimary:   int(l.GetMatchBallPrimary()),
			MatchBallSecondary: int(l.GetMatchBallSecondary()),
		}
		levels[i].Prize.PrizePence = l.GetPrizePence()
	}

	d := data.DrawDetails{}
	d.DrawResult.GameID = int(pbDraw.GetDrawResult().GetGameId())
	d.DrawResult.DrawNo = int(pbDraw.GetDrawResult().GetDrawNo())
	d.DrawResult.DrawDate = t
	d.DrawResult.DrawnNumbers.DrawnNumbers.PrimaryNumbers = primarySlice
	d.DrawResult.DrawnNumbers.DrawnNumbers.SecondaryNumbers = secondarySlice
	d.PrizeBreakdown.PrizeLevels = levels

	return d
}

func intToInt32Slice(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		out[i] = int32(v)
	}
	return out
}
