package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/data"
	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/evaluator"
	analyserPb "github.com/Rosalita/distributed-lottery-analyser/protos/generated/analyser"
)

func TestNewCoordinator(t *testing.T) {
	config := evaluator.ThunderballConfig // PrimaryCount=39, PrimarySelect=5, SecondaryCount=14, SecondarySelect=1 -> 8,060,598
	draws := map[int]data.DrawDetails{
		1: {
			DrawResult: data.DrawResult{
				GameID: 1,
				DrawNo: 1,
				DrawnNumbers: data.DrawnNumbersWrapper{},
			},
		},
	}

	chunkSize := int64(1_000_000)
	c := NewCoordinator(config, draws, chunkSize, 5, 30*time.Second)

	expectedChunks := int(8_060_598/chunkSize) + 1
	if len(c.chunks) != expectedChunks {
		t.Fatalf("expected %d chunks, got %d", expectedChunks, len(c.chunks))
	}

	if c.totalCombinations != 8_060_598 {
		t.Errorf("expected totalCombinations 8,060,598, got %d", c.totalCombinations)
	}

	if c.chunks[0].StartRank != 0 || c.chunks[0].EndRank != chunkSize {
		t.Errorf("unexpected first chunk bounds: [%d, %d)", c.chunks[0].StartRank, c.chunks[0].EndRank)
	}

	lastChunk := c.chunks[len(c.chunks)-1]
	if lastChunk.EndRank != 8_060_598 {
		t.Errorf("unexpected last chunk end rank: %d", lastChunk.EndRank)
	}
}

func TestRegisterWorker(t *testing.T) {
	config := evaluator.ThunderballConfig
	draws := map[int]data.DrawDetails{
		3923: {
			DrawResult: data.DrawResult{
				GameID:   3,
				DrawNo:   3923,
				DrawDate: time.Date(2026, 6, 21, 20, 0, 0, 0, time.UTC),
			},
		},
	}

	c := NewCoordinator(config, draws, 1_000_000, 5, 30*time.Second)
	ctx := context.Background()

	// Error case: empty worker ID
	_, err := c.RegisterWorker(ctx, &analyserPb.RegisterWorkerRequest{WorkerId: ""})
	if err == nil {
		t.Error("expected error when registering with empty worker ID")
	}

	resp, err := c.RegisterWorker(ctx, &analyserPb.RegisterWorkerRequest{WorkerId: "worker-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.GameConfig.Name != "thunderball" {
		t.Errorf("expected game name 'thunderball', got '%s'", resp.GameConfig.Name)
	}

	if len(resp.Draws) != 1 {
		t.Fatalf("expected 1 draw, got %d", len(resp.Draws))
	}

	if resp.Draws[0].DrawResult.DrawNo != 3923 {
		t.Errorf("expected draw no 3923, got %d", resp.Draws[0].DrawResult.DrawNo)
	}
}

func TestGetWorkAndTimeout(t *testing.T) {
	config := evaluator.ThunderballConfig // 8,060,598
	draws := map[int]data.DrawDetails{}

	// Coordinator with very short timeout for testing
	timeout := 10 * time.Millisecond
	c := NewCoordinator(config, draws, 1_000_000, 5, timeout)
	ctx := context.Background()

	// 1. Get first chunk for worker 1
	w1Resp, err := c.GetWork(ctx, &analyserPb.GetWorkRequest{WorkerId: "worker-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w1Resp.StartRank != 0 || w1Resp.EndRank != 1_000_000 {
		t.Errorf("expected first chunk [0, 1000000), got [%d, %d)", w1Resp.StartRank, w1Resp.EndRank)
	}

	// First chunk should be InProgress for worker-1
	if c.chunks[0].State != StateInProgress || c.chunks[0].WorkerID != "worker-1" {
		t.Error("chunk 0 state was not updated correctly")
	}

	// 2. Get next chunk for worker-2 immediately (no timeout yet)
	w2Resp, err := c.GetWork(ctx, &analyserPb.GetWorkRequest{WorkerId: "worker-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w2Resp.StartRank != 1_000_000 || w2Resp.EndRank != 2_000_000 {
		t.Errorf("expected second chunk [1000000, 2000000), got [%d, %d)", w2Resp.StartRank, w2Resp.EndRank)
	}

	// Wait for worker-1's chunk to timeout
	time.Sleep(15 * time.Millisecond)

	// 3. Worker-3 requests work. All chunks are either InProgress or Unassigned (some further down).
	// Because worker-1's chunk timed out, it should be reassigned to worker-3.
	w3Resp, err := c.GetWork(ctx, &analyserPb.GetWorkRequest{WorkerId: "worker-3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Note: Our implementation checks Unassigned first, then checks timed out chunks.
	// Since chunk 2 (2,000,000 to 3,000,000) is still StateUnassigned, GetWork should distribute that first!
	if w3Resp.StartRank != 2_000_000 {
		t.Errorf("expected worker-3 to get next unassigned chunk [2000000, 3000000), got start rank %d", w3Resp.StartRank)
	}

	// Let's exhaust all unassigned chunks. There are 9 chunks in total.
	// We have assigned chunk 0 (w1), chunk 1 (w2), chunk 2 (w3). Let's assign chunks 3 to 8.
	for i := 3; i < 9; i++ {
		_, err := c.GetWork(ctx, &analyserPb.GetWorkRequest{WorkerId: "worker-batch"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// At this point, all chunks are InProgress. Chunk 0 has definitely timed out.
	// Let's ask for work again. It should reassign the timed-out Chunk 0.
	reassignResp, err := c.GetWork(ctx, &analyserPb.GetWorkRequest{WorkerId: "worker-reassign"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if reassignResp.StartRank != 0 || reassignResp.EndRank != 1_000_000 {
		t.Errorf("expected reassigned first chunk [0, 1000000), got [%d, %d)", reassignResp.StartRank, reassignResp.EndRank)
	}
	if c.chunks[0].WorkerID != "worker-reassign" {
		t.Errorf("expected chunk worker ID to be updated, got %s", c.chunks[0].WorkerID)
	}
}

func TestReportResultAndProgress(t *testing.T) {
	config := evaluator.ThunderballConfig
	draws := map[int]data.DrawDetails{}
	c := NewCoordinator(config, draws, 1_000_000, 5, 30*time.Second)
	ctx := context.Background()

	// Assign first chunk to worker-1
	_, _ = c.GetWork(ctx, &analyserPb.GetWorkRequest{WorkerId: "worker-1"})

	// Report results for chunk 0
	report := &analyserPb.ReportResultRequest{
		WorkerId:  "worker-1",
		StartRank: 0,
		EndRank:   1_000_000,
		TopTickets: []*analyserPb.Ticket{
			{
				PrimaryNumbers:   []int32{1, 2, 3, 4, 5},
				SecondaryNumbers: []int32{6},
				TotalPrizeCents:  100_000,
			},
		},
	}

	resp, err := c.ReportResult(ctx, report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got message: %s", resp.Message)
	}

	if c.chunks[0].State != StateCompleted {
		t.Error("expected chunk to be completed")
	}

	// Verify top tickets merged
	top := c.GetTopTickets()
	if len(top) != 1 {
		t.Fatalf("expected 1 top ticket, got %d", len(top))
	}

	if top[0].TotalPrizeCents != 100_000 {
		t.Errorf("expected prize 100,000, got %d", top[0].TotalPrizeCents)
	}

	completed, total := c.Progress()
	if completed != 1 {
		t.Errorf("expected 1 completed chunk, got %d", completed)
	}
	if total != 9 {
		t.Errorf("expected 9 total chunks, got %d", total)
	}

	if c.AllDone() {
		t.Error("expected AllDone to be false since only 1 chunk out of 9 completed")
	}
}
