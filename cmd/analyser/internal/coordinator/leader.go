package coordinator

import (
	"context"
	"sync"
	"time"

	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/data"
	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/evaluator"
	analyserPb "github.com/Rosalita/distributed-lottery-analyser/protos/generated/analyser"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ChunkState int

const (
	StateUnassigned ChunkState = iota
	StateInProgress
	StateCompleted
)

// Chunk represents a segment of the combination space.
type Chunk struct {
	ID         int
	StartRank  int64
	EndRank    int64
	State      ChunkState
	WorkerID   string
	LastActive time.Time
}

// WorkerInfo tracks active workers registered with the leader.
type WorkerInfo struct {
	ID          string
	LastSeen    time.Time
	ActiveChunk *Chunk
}

// Coordinator manages the distributed solving process.
type Coordinator struct {
	analyserPb.UnimplementedAnalyserServer
	mu                sync.Mutex
	gameConfig        evaluator.GameConfig
	draws             []data.DrawDetails
	fastDraws         []evaluator.FastDraw
	totalCombinations int64
	chunkSize         int64
	chunks            []*Chunk
	workers           map[string]*WorkerInfo
	topTickets        *evaluator.TopTickets
	limit             int
	workerTimeout     time.Duration
}

// NewCoordinator initializes a new Coordinator.
func NewCoordinator(config evaluator.GameConfig, drawMap map[int]data.DrawDetails, chunkSize int64, limit int, workerTimeout time.Duration) *Coordinator {
	// Convert draw map to slice for order stability and iteration ease
	var draws []data.DrawDetails
	var fastDraws []evaluator.FastDraw
	for _, d := range drawMap {
		draws = append(draws, d)
		fastDraws = append(fastDraws, evaluator.NewFastDraw(d, config.Name))
	}

	var totalCombinations int64
	if config.SecondarySelect > 0 {
		totalCombinations = evaluator.Choose(config.PrimaryCount, config.PrimarySelect) * evaluator.Choose(config.SecondaryCount, config.SecondarySelect)
	} else {
		totalCombinations = evaluator.Choose(config.PrimaryCount, config.PrimarySelect)
	}

	var chunks []*Chunk
	var chunkID int
	for start := int64(0); start < totalCombinations; start += chunkSize {
		end := start + chunkSize
		if end > totalCombinations {
			end = totalCombinations
		}
		chunks = append(chunks, &Chunk{
			ID:        chunkID,
			StartRank: start,
			EndRank:   end,
			State:     StateUnassigned,
		})
		chunkID++
	}

	return &Coordinator{
		gameConfig:        config,
		draws:             draws,
		fastDraws:         fastDraws,
		totalCombinations: totalCombinations,
		chunkSize:         chunkSize,
		chunks:            chunks,
		workers:           make(map[string]*WorkerInfo),
		topTickets:        evaluator.NewTopTickets(limit),
		limit:             limit,
		workerTimeout:     workerTimeout,
	}
}

// RegisterWorker handles worker registration and returns game parameters and historical draw data.
func (c *Coordinator) RegisterWorker(ctx context.Context, req *analyserPb.RegisterWorkerRequest) (*analyserPb.RegisterWorkerResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	workerID := req.GetWorkerId()
	if workerID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "worker_id must be provided")
	}

	c.workers[workerID] = &WorkerInfo{
		ID:       workerID,
		LastSeen: time.Now(),
	}

	pbGameConfig := &analyserPb.GameConfig{
		Name:            c.gameConfig.Name,
		PrimaryCount:    int32(c.gameConfig.PrimaryCount),
		PrimarySelect:   int32(c.gameConfig.PrimarySelect),
		SecondaryCount:  int32(c.gameConfig.SecondaryCount),
		SecondarySelect: int32(c.gameConfig.SecondarySelect),
	}

	pbDraws := make([]*analyserPb.DrawDetails, len(c.draws))
	for i, d := range c.draws {
		pbLevels := make([]*analyserPb.PrizeLevel, len(d.PrizeBreakdown.PrizeLevels))
		for j, l := range d.PrizeBreakdown.PrizeLevels {
			pbLevels[j] = &analyserPb.PrizeLevel{
				MatchLabel:         l.MatchLabel,
				MatchBallPrimary:   int32(l.MatchBallPrimary),
				MatchBallSecondary: int32(l.MatchBallSecondary),
				PrizeCents:         l.Prize.PrizeCents,
			}
		}
		pbDraws[i] = &analyserPb.DrawDetails{
			DrawResult: &analyserPb.DrawResult{
				GameId:   int32(d.DrawResult.GameID),
				DrawNo:   int32(d.DrawResult.DrawNo),
				DrawDate: d.DrawResult.DrawDate.Format(time.RFC3339),
				DrawnNumbers: &analyserPb.DrawnNumbers{
					PrimaryNumbers:   intToInt32Slice(d.DrawResult.DrawnNumbers.DrawnNumbers.PrimaryNumbers),
					SecondaryNumbers: intToInt32Slice(d.DrawResult.DrawnNumbers.DrawnNumbers.SecondaryNumbers),
				},
			},
			PrizeBreakdown: &analyserPb.PrizeBreakdown{
				PrizeLevels: pbLevels,
			},
		}
	}

	return &analyserPb.RegisterWorkerResponse{
		GameConfig:        pbGameConfig,
		Draws:             pbDraws,
		TotalCombinations: c.totalCombinations,
	}, nil
}

// GetWork distributes a range chunk of work to the requesting worker.
func (c *Coordinator) GetWork(ctx context.Context, req *analyserPb.GetWorkRequest) (*analyserPb.GetWorkResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	workerID := req.GetWorkerId()
	if workerID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "worker_id must be provided")
	}

	w, exists := c.workers[workerID]
	if !exists {
		w = &WorkerInfo{
			ID:       workerID,
			LastSeen: time.Now(),
		}
		c.workers[workerID] = w
	}
	w.LastSeen = time.Now()

	// 1. First assign unassigned chunks
	for _, chunk := range c.chunks {
		if chunk.State == StateUnassigned {
			chunk.State = StateInProgress
			chunk.WorkerID = workerID
			chunk.LastActive = time.Now()
			w.ActiveChunk = chunk
			return &analyserPb.GetWorkResponse{
				StartRank: chunk.StartRank,
				EndRank:   chunk.EndRank,
			}, nil
		}
	}

	// 2. Reassign timed out in-progress chunks
	for _, chunk := range c.chunks {
		if chunk.State == StateInProgress && time.Since(chunk.LastActive) > c.workerTimeout {
			chunk.WorkerID = workerID
			chunk.LastActive = time.Now()
			w.ActiveChunk = chunk
			return &analyserPb.GetWorkResponse{
				StartRank: chunk.StartRank,
				EndRank:   chunk.EndRank,
			}, nil
		}
	}

	// 3. No work remains
	return &analyserPb.GetWorkResponse{
		NoMoreWork: true,
	}, nil
}

// ReportResult processes completion of a chunk and merges the reported top tickets.
func (c *Coordinator) ReportResult(ctx context.Context, req *analyserPb.ReportResultRequest) (*analyserPb.ReportResultResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	workerID := req.GetWorkerId()
	startRank := req.GetStartRank()
	endRank := req.GetEndRank()

	w, exists := c.workers[workerID]
	if exists {
		w.LastSeen = time.Now()
	}

	var targetChunk *Chunk
	for _, chunk := range c.chunks {
		if chunk.StartRank == startRank && chunk.EndRank == endRank {
			targetChunk = chunk
			break
		}
	}

	if targetChunk == nil {
		return &analyserPb.ReportResultResponse{
			Success: false,
			Message: "chunk not found",
		}, nil
	}

	if targetChunk.State != StateCompleted {
		targetChunk.State = StateCompleted
		targetChunk.WorkerID = workerID
		if w != nil && w.ActiveChunk == targetChunk {
			w.ActiveChunk = nil
		}
	}

	for _, t := range req.GetTopTickets() {
		primary := make([]int, len(t.GetPrimaryNumbers()))
		for idx, val := range t.GetPrimaryNumbers() {
			primary[idx] = int(val)
		}
		secondary := make([]int, len(t.GetSecondaryNumbers()))
		for idx, val := range t.GetSecondaryNumbers() {
			secondary[idx] = int(val)
		}
		c.topTickets.Add(t.GetTotalPrizeCents(), primary, secondary)
	}

	return &analyserPb.ReportResultResponse{
		Success: true,
		Message: "results recorded successfully",
	}, nil
}

// AllDone checks if all chunks have been successfully completed.
func (c *Coordinator) AllDone() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, chunk := range c.chunks {
		if chunk.State != StateCompleted {
			return false
		}
	}
	return true
}

// GetTopTickets returns a copy of the current top accumulated tickets.
func (c *Coordinator) GetTopTickets() []evaluator.Ticket {
	c.mu.Lock()
	defer c.mu.Unlock()

	res := make([]evaluator.Ticket, len(c.topTickets.Tickets))
	copy(res, c.topTickets.Tickets)
	return res
}

// Progress returns the count of completed and total chunks.
func (c *Coordinator) Progress() (completed, total int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	completed = 0
	total = len(c.chunks)
	for _, chunk := range c.chunks {
		if chunk.State == StateCompleted {
			completed++
		}
	}
	return completed, total
}

func intToInt32Slice(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		out[i] = int32(v)
	}
	return out
}
