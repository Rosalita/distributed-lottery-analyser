package worker

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/evaluator"
	analyserPb "github.com/Rosalita/distributed-lottery-analyser/protos/generated/analyser"

	"google.golang.org/grpc"
)

type mockLeaderServer struct {
	analyserPb.UnimplementedAnalyserServer
	registerCalled bool
	getWorkCount   int
	reports        []*analyserPb.ReportResultRequest
}

func (m *mockLeaderServer) RegisterWorker(ctx context.Context, req *analyserPb.RegisterWorkerRequest) (*analyserPb.RegisterWorkerResponse, error) {
	m.registerCalled = true
	return &analyserPb.RegisterWorkerResponse{
		GameConfig: &analyserPb.GameConfig{
			Name:            "thunderball",
			PrimaryCount:    39,
			PrimarySelect:   5,
			SecondaryCount:  14,
			SecondarySelect: 1,
		},
		Draws: []*analyserPb.DrawDetails{
			{
				DrawResult: &analyserPb.DrawResult{
					GameId:   3,
					DrawNo:   1,
					DrawDate: time.Now().Format(time.RFC3339),
					DrawnNumbers: &analyserPb.DrawnNumbers{
						PrimaryNumbers:   []int32{1, 2, 3, 4, 5},
						SecondaryNumbers: []int32{6},
					},
				},
				PrizeBreakdown: &analyserPb.PrizeBreakdown{
					PrizeLevels: []*analyserPb.PrizeLevel{
						{
							MatchLabel:         "5+1",
							MatchBallPrimary:   5,
							MatchBallSecondary: 1,
							PrizePence:         500_000_000,
						},
					},
				},
			},
		},
		TotalCombinations: 8_060_598,
	}, nil
}

func (m *mockLeaderServer) GetWork(ctx context.Context, req *analyserPb.GetWorkRequest) (*analyserPb.GetWorkResponse, error) {
	m.getWorkCount++
	if m.getWorkCount == 1 {
		return &analyserPb.GetWorkResponse{
			StartRank:  0,
			EndRank:    10,
			NoMoreWork: false,
		}, nil
	}
	return &analyserPb.GetWorkResponse{
		NoMoreWork: true,
	}, nil
}

func (m *mockLeaderServer) ReportResult(ctx context.Context, req *analyserPb.ReportResultRequest) (*analyserPb.ReportResultResponse, error) {
	m.reports = append(m.reports, req)
	return &analyserPb.ReportResultResponse{
		Success: true,
	}, nil
}

func TestRunWorker(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	server := grpc.NewServer()
	mock := &mockLeaderServer{}
	analyserPb.RegisterAnalyserServer(server, mock)

	go func() {
		_ = server.Serve(lis)
	}()
	defer server.Stop()

	err = RunWorker(lis.Addr().String(), 5)
	if err != nil {
		t.Fatalf("worker failed: %v", err)
	}

	if !mock.registerCalled {
		t.Error("RegisterWorker was not called")
	}

	if mock.getWorkCount != 2 {
		t.Errorf("expected 2 GetWork calls, got %d", mock.getWorkCount)
	}

	if len(mock.reports) != 1 {
		t.Fatalf("expected 1 reported chunk, got %d", len(mock.reports))
	}

	r := mock.reports[0]
	if r.StartRank != 0 || r.EndRank != 10 {
		t.Errorf("unexpected reported range: [%d, %d)", r.StartRank, r.EndRank)
	}
}

func TestWorkerSetForLifeMapping(t *testing.T) {
	pbDraw := &analyserPb.DrawDetails{
		DrawResult: &analyserPb.DrawResult{
			GameId:   3, // Set For Life
			DrawNo:   742,
			DrawDate: time.Now().Format(time.RFC3339),
			DrawnNumbers: &analyserPb.DrawnNumbers{
				PrimaryNumbers:   []int32{8, 14, 20, 23, 32},
				SecondaryNumbers: []int32{1},
			},
		},
		PrizeBreakdown: &analyserPb.PrizeBreakdown{
			PrizeLevels: []*analyserPb.PrizeLevel{
				{
					MatchLabel:         "Match 5 + Life Ball",
					MatchBallPrimary:   5,
					MatchBallSecondary: 1,
					PrizePence:         0, // null/0 in JSON
					DrawRound:          "ONE",
				},
				{
					MatchLabel:         "Match 5",
					MatchBallPrimary:   5,
					MatchBallSecondary: 0,
					PrizePence:         0, // null/0 in JSON
					DrawRound:          "ONE",
				},
			},
		},
	}

	d := mapDraw(pbDraw)

	if d.DrawResult.GameID != 3 {
		t.Errorf("Expected GameID to be 3, got %d", d.DrawResult.GameID)
	}

	fd := evaluator.NewFastDraw(d, "setforlife")

	p5_1 := fd.PrizeMatrix[5][1]
	if p5_1 != 360000000 {
		t.Errorf("Expected 5+1 prize in matrix to be 360000000, got %d", p5_1)
	}

	p5_0 := fd.PrizeMatrix[5][0]
	if p5_0 != 12000000 {
		t.Errorf("Expected 5+0 prize in matrix to be 12000000, got %d", p5_0)
	}
}

