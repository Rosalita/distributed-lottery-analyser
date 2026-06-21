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
		drawNumbersAdd   []int
		drawSecondaryAdd []int
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
		"lotto match 4 + bonus": {
			// Player ticket [1, 2, 3, 4, 5, 6] matches primary 1, 2, 3, 4 and bonus 6.
			// Because matchPrimary is 4 (not 5), bonus ball matching should not affect the tier,
			// and it should correctly receive the Match 4 prize.
			config:           LottoConfig,
			drawNumbers:      []int{1, 2, 3, 4, 10, 11},
			drawSecondary:    []int{6},
			rank:             0,
			expectedTicketP:  []int{1, 2, 3, 4, 5, 6},
			expectedTicketS:  nil,
			expectedPrize:    14000,
		},
		"lotto two round": {
			// Player ticket [1, 2, 3, 4, 5, 6] matches:
			// Round 1: [1, 2, 3, 4, 10, 11] -> Match 4 (14000 pence)
			// Round 2: [1, 2, 3, 13, 14, 15] -> Match 3 (3000 pence)
			// Total prize: 14000 + 3000 = 17000 pence.
			config:           LottoConfig,
			drawNumbers:      []int{1, 2, 3, 4, 10, 11},
			drawSecondary:    []int{12},
			drawNumbersAdd:   []int{1, 2, 3, 13, 14, 15},
			drawSecondaryAdd: []int{16},
			rank:             0,
			expectedTicketP:  []int{1, 2, 3, 4, 5, 6},
			expectedTicketS:  nil,
			expectedPrize:    17000,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			details := data.DrawDetails{}
			details.DrawResult.DrawNo = 100
			details.DrawResult.GameID = 6 // Lotto
			details.DrawResult.DrawnNumbers.DrawnNumbers.PrimaryNumbers = tc.drawNumbers
			details.DrawResult.DrawnNumbers.DrawnNumbers.SecondaryNumbers = tc.drawSecondary

			if tc.drawNumbersAdd != nil {
				details.DrawResult.DrawnNumbers.DrawnNumbersAdditional = &struct {
					PrimaryNumbers   []int `json:"primaryNumbers"`
					SecondaryNumbers []int `json:"secondaryNumbers"`
				}{
					PrimaryNumbers:   tc.drawNumbersAdd,
					SecondaryNumbers: tc.drawSecondaryAdd,
				}
			}

			details.PrizeBreakdown.PrizeLevels = []data.PrizeLevel{
				{DrawRound: "ONE", MatchBallPrimary: 6, MatchBallSecondary: 0},
				{DrawRound: "ONE", MatchBallPrimary: 5, MatchBallSecondary: 1},
				{DrawRound: "ONE", MatchBallPrimary: 5, MatchBallSecondary: 0},
				{DrawRound: "ONE", MatchBallPrimary: 4, MatchBallSecondary: 0},
				{DrawRound: "ONE", MatchBallPrimary: 3, MatchBallSecondary: 0},
				{DrawRound: "TWO", MatchBallPrimary: 3, MatchBallSecondary: 0},
			}
			details.PrizeBreakdown.PrizeLevels[0].Prize.PrizePence = 1000000
			details.PrizeBreakdown.PrizeLevels[1].Prize.PrizePence = 500000
			details.PrizeBreakdown.PrizeLevels[2].Prize.PrizePence = 10000
			details.PrizeBreakdown.PrizeLevels[3].Prize.PrizePence = 14000
			details.PrizeBreakdown.PrizeLevels[4].Prize.PrizePence = 3000
			details.PrizeBreakdown.PrizeLevels[5].Prize.PrizePence = 3000

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
			if bestTicket.TotalPrizePence != tc.expectedPrize {
				t.Errorf("expected prize %d, got %d", tc.expectedPrize, bestTicket.TotalPrizePence)
			}
		})
	}
}
