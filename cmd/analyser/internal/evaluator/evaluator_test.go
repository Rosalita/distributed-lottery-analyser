package evaluator

import (
	"reflect"
	"testing"

	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/data"
)

func TestEvaluateRange(t *testing.T) {
	tests := map[string]struct {
		config           GameConfig
		drawNumbers      []int
		drawSecondary    []int
		rank             int64
		expectedTicketP  []int
		expectedTicketS  []int
		expectedPrize    int64
	}{
		"lotto match 6": {
			config:           LottoConfig,
			drawNumbers:      []int{1, 2, 3, 4, 5, 6},
			drawSecondary:    []int{7},
			rank:             0, // Rank 0 maps to [1, 2, 3, 4, 5, 6]
			expectedTicketP:  []int{1, 2, 3, 4, 5, 6},
			expectedTicketS:  nil,
			expectedPrize:    1000000,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			details := data.DrawDetails{}
			details.DrawResult.DrawNo = 100
			details.DrawResult.DrawnNumbers.DrawnNumbers.PrimaryNumbers = tc.drawNumbers
			details.DrawResult.DrawnNumbers.DrawnNumbers.SecondaryNumbers = tc.drawSecondary

			details.PrizeBreakdown.PrizeLevels = []data.PrizeLevel{
				{MatchBallPrimary: 6, MatchBallSecondary: 0},
				{MatchBallPrimary: 5, MatchBallSecondary: 1},
				{MatchBallPrimary: 5, MatchBallSecondary: 0},
			}
			details.PrizeBreakdown.PrizeLevels[0].Prize.PrizeCents = 1000000
			details.PrizeBreakdown.PrizeLevels[1].Prize.PrizeCents = 500000
			details.PrizeBreakdown.PrizeLevels[2].Prize.PrizeCents = 10000

			fd := NewFastDraw(details, tc.config.Name)

			tickets := EvaluateRange(tc.rank, tc.rank+1, tc.config, []FastDraw{fd}, 5)
			if len(tickets) != 1 {
				t.Fatalf("expected 1 ticket in results, got %d", len(tickets))
			}

			bestTicket := tickets[0]
			if !reflect.DeepEqual(bestTicket.PrimaryNumbers, tc.expectedTicketP) {
				t.Errorf("expected ticket primary %+v, got %+v", tc.expectedTicketP, bestTicket.PrimaryNumbers)
			}
			if !reflect.DeepEqual(bestTicket.SecondaryNumbers, tc.expectedTicketS) {
				t.Errorf("expected ticket secondary %+v, got %+v", tc.expectedTicketS, bestTicket.SecondaryNumbers)
			}
			if bestTicket.TotalPrizeCents != tc.expectedPrize {
				t.Errorf("expected prize %d, got %d", tc.expectedPrize, bestTicket.TotalPrizeCents)
			}
		})
	}
}
