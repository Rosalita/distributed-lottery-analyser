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

func TestSetForLifePrizes(t *testing.T) {
	d := data.DrawDetails{}
	d.DrawResult.GameID = 3 // Set For Life
	d.PrizeBreakdown.PrizeLevels = []data.PrizeLevel{
		{
			DrawRound:          "ONE",
			MatchBallPrimary:   5,
			MatchBallSecondary: 1,
		},
		{
			DrawRound:          "ONE",
			MatchBallPrimary:   5,
			MatchBallSecondary: 0,
		},
	}
	// Note: PrizePence defaults to 0 (which simulates null/0 prizeCents from JSON)

	p5_1 := d.GetPrizeForRound(5, 1, "ONE")
	if p5_1 != 360000000 {
		t.Errorf("Expected 5+1 prize to be 360000000 (360 million pence), got %d", p5_1)
	}

	p5_0 := d.GetPrizeForRound(5, 0, "ONE")
	if p5_0 != 12000000 {
		t.Errorf("Expected 5+0 prize to be 12000000 (12 million pence), got %d", p5_0)
	}
}

func TestLoadSetForLifeDataPrizes(t *testing.T) {
	// Navigate locally to the getdrawhistory/data folder
	baseDataDir := "../../../getdrawhistory/data"
	draws, err := data.LoadGameData("setforlife", baseDataDir)
	if err != nil {
		t.Fatalf("Failed to load game data: %v", err)
	}

	if len(draws) == 0 {
		t.Fatalf("No draws loaded for setforlife")
	}

	t.Logf("Loaded %d draws", len(draws))

	for drawNo, d := range draws {
		p5_1 := d.GetPrizeForRound(5, 1, "ONE")
		p5_0 := d.GetPrizeForRound(5, 0, "ONE")

		// Both should be overridden to their correct annuity values
		if p5_1 != 360000000 {
			t.Errorf("Draw %d: expected 5+1 prize to be 360000000, got %d", drawNo, p5_1)
		}
		if p5_0 != 12000000 {
			t.Errorf("Draw %d: expected 5+0 prize to be 12000000, got %d", drawNo, p5_0)
		}
	}
}

func TestSetForLifeTopTicketEvaluation(t *testing.T) {
	baseDataDir := "../../../getdrawhistory/data"
	drawsMap, err := data.LoadGameData("setforlife", baseDataDir)
	if err != nil {
		t.Fatalf("Failed to load game data: %v", err)
	}

	var fastDraws []FastDraw
	for _, d := range drawsMap {
		fastDraws = append(fastDraws, NewFastDraw(d, "setforlife"))
	}

	// Look for the rank of the winning ticket of Draw 742: [8, 14, 20, 23, 32], Life Ball 1
	targetP := []int{8, 14, 20, 23, 32}
	targetS := []int{1}

	var foundRank int64 = -1
	totalCombinations := Choose(47, 5) * Choose(10, 1)
	for rank := int64(0); rank < totalCombinations; rank++ {
		pSlice, sSlice := UnrankTicket(rank, SetForLifeConfig)
		if reflect.DeepEqual(pSlice, targetP) && reflect.DeepEqual(sSlice, targetS) {
			foundRank = rank
			break
		}
	}

	if foundRank == -1 {
		t.Fatalf("Failed to find rank for ticket %+v, %+v", targetP, targetS)
	}

	t.Logf("Found rank: %d", foundRank)

	tickets := EvaluateRange(foundRank, foundRank+1, SetForLifeConfig, fastDraws, 5)
	if len(tickets) != 1 {
		t.Fatalf("Expected 1 ticket, got %d", len(tickets))
	}

	ticket := tickets[0]
	t.Logf("Ticket: %+v, %+v | Total Earnings: %d pence (£%d)", ticket.PrimaryNumbers, ticket.SecondaryNumbers, ticket.TotalPrizePence, ticket.TotalPrizePence/100)

	if ticket.TotalPrizePence < 360000000 {
		t.Errorf("Expected total prize to be at least 360 million pence (£3.6M), got %d", ticket.TotalPrizePence)
	}
}

func TestSetForLifeRangeEvaluation(t *testing.T) {
	baseDataDir := "../../../getdrawhistory/data"
	drawsMap, err := data.LoadGameData("setforlife", baseDataDir)
	if err != nil {
		t.Fatalf("Failed to load game data: %v", err)
	}

	var fastDraws []FastDraw
	for _, d := range drawsMap {
		fastDraws = append(fastDraws, NewFastDraw(d, "setforlife"))
	}

	tickets := EvaluateRange(0, 2000000, SetForLifeConfig, fastDraws, 5)
	t.Logf("Top 5 tickets in chunk [0, 2000000):")
	for idx, tk := range tickets {
		t.Logf("%d. Primary: %v, Secondary: %v | Total Earnings: %d pence (£%d)", idx+1, tk.PrimaryNumbers, tk.SecondaryNumbers, tk.TotalPrizePence, tk.TotalPrizePence/100)
	}

	if len(tickets) == 0 {
		t.Fatalf("Expected tickets, got 0")
	}

	if tickets[0].TotalPrizePence < 360000000 {
		t.Errorf("Top ticket did not have the jackpot prize! Got %d", tickets[0].TotalPrizePence)
	}
}




