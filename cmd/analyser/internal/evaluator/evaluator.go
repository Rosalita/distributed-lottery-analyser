package evaluator

import (
	"math/bits"

	"github.com/Rosalita/distributed-lottery-analyser/cmd/analyser/internal/data"
)

// GameConfig defines the lottery game combination rules.
type GameConfig struct {
	Name            string
	PrimaryCount    int
	PrimarySelect   int
	SecondaryCount  int
	SecondarySelect int
}

var (
	LottoConfig = GameConfig{
		Name:            "lotto",
		PrimaryCount:    59,
		PrimarySelect:   6,
		SecondaryCount:  0,
		SecondarySelect: 0,
	}
	EuroMillionsConfig = GameConfig{
		Name:            "euromillions",
		PrimaryCount:    50,
		PrimarySelect:   5,
		SecondaryCount:  12,
		SecondarySelect: 2,
	}
	ThunderballConfig = GameConfig{
		Name:            "thunderball",
		PrimaryCount:    39,
		PrimarySelect:   5,
		SecondaryCount:  14,
		SecondarySelect: 1,
	}
	SetForLifeConfig = GameConfig{
		Name:            "setforlife",
		PrimaryCount:    47,
		PrimarySelect:   5,
		SecondaryCount:  10,
		SecondarySelect: 1,
	}
)

// GetGameConfig returns the GameConfig for a given game name.
func GetGameConfig(name string) (GameConfig, bool) {
	switch name {
	case "lotto":
		return LottoConfig, true
	case "euromillions":
		return EuroMillionsConfig, true
	case "thunderball":
		return ThunderballConfig, true
	case "setforlife":
		return SetForLifeConfig, true
	default:
		return GameConfig{}, false
	}
}

// Ticket represents a played combination and its calculated historical profit.
type Ticket struct {
	PrimaryNumbers   []int
	SecondaryNumbers []int
	TotalPrizeCents  int64
}

// FastDraw is a high-performance representation of a historical draw.
type FastDraw struct {
	DrawNo          int
	IsLottoTwoRound bool
	PrimaryMask     uint64
	SecondaryMask   uint64
	PrizeMatrix     [7][3]int64 // PrizeMatrix[matchPrimary][matchSecondary] -> prize cents
	PrimaryMask2    uint64
	SecondaryMask2  uint64
	PrizeMatrix2    [7][3]int64 // PrizeMatrix2[matchPrimary][matchSecondary] -> prize cents for Round 2
}

// NewFastDraw converts standard DrawDetails to a FastDraw.
func NewFastDraw(d data.DrawDetails, gameName string) FastDraw {
	fd := FastDraw{
		DrawNo: d.DrawResult.DrawNo,
	}
	fd.PrimaryMask = SliceToMask(d.DrawResult.DrawnNumbers.DrawnNumbers.PrimaryNumbers)
	fd.SecondaryMask = SliceToMask(d.DrawResult.DrawnNumbers.DrawnNumbers.SecondaryNumbers)

	// Pre-fill the 2D matrix for match primary (0 to 6) and match secondary (0 to 2) for Round 1
	for p := 0; p < 7; p++ {
		for s := 0; s < 3; s++ {
			fd.PrizeMatrix[p][s] = d.GetPrizeForRound(p, s, "ONE")
		}
	}

	if gameName == "lotto" && d.DrawResult.DrawnNumbers.DrawnNumbersAdditional != nil {
		fd.IsLottoTwoRound = true
		fd.PrimaryMask2 = SliceToMask(d.DrawResult.DrawnNumbers.DrawnNumbersAdditional.PrimaryNumbers)
		fd.SecondaryMask2 = SliceToMask(d.DrawResult.DrawnNumbers.DrawnNumbersAdditional.SecondaryNumbers)
		for p := 0; p < 7; p++ {
			for s := 0; s < 3; s++ {
				fd.PrizeMatrix2[p][s] = d.GetPrizeForRound(p, s, "TWO")
			}
		}
	}

	return fd
}

// TopTickets tracks the top K highest-earning tickets using a sorted slice.
type TopTickets struct {
	Tickets []Ticket
	Limit   int
}

// NewTopTickets creates a new TopTickets tracker.
func NewTopTickets(limit int) *TopTickets {
	return &TopTickets{
		Tickets: make([]Ticket, 0, limit+1),
		Limit:   limit,
	}
}

// Add inserts a ticket into the top list if its payout warrants it, maintaining sorted descending order.
// Slice copying is deferred until insertion to avoid unnecessary memory allocations.
func (tt *TopTickets) Add(totalPrize int64, primarySlice, secondarySlice []int) {
	// If the list is full and the new ticket is not better than the worst in our list, skip it.
	if len(tt.Tickets) >= tt.Limit && totalPrize <= tt.Tickets[len(tt.Tickets)-1].TotalPrizeCents {
		return
	}

	// Find insertion index
	idx := len(tt.Tickets)
	for i, existing := range tt.Tickets {
		if totalPrize > existing.TotalPrizeCents {
			idx = i
			break
		}
	}

	// Copy slices now that we know we are keeping this ticket
	pSlice := make([]int, len(primarySlice))
	copy(pSlice, primarySlice)

	var sSlice []int
	if len(secondarySlice) > 0 {
		sSlice = make([]int, len(secondarySlice))
		copy(sSlice, secondarySlice)
	}

	t := Ticket{
		PrimaryNumbers:   pSlice,
		SecondaryNumbers: sSlice,
		TotalPrizeCents:  totalPrize,
	}

	// Insert into slice
	tt.Tickets = append(tt.Tickets, Ticket{})
	copy(tt.Tickets[idx+1:], tt.Tickets[idx:])
	tt.Tickets[idx] = t

	if len(tt.Tickets) > tt.Limit {
		tt.Tickets = tt.Tickets[:tt.Limit]
	}
}

// EvaluateRange simulates playing every combination in the range [startRank, endRank) against historical draws.
// Returns the sorted list of top-performing tickets.
func EvaluateRange(startRank, endRank int64, config GameConfig, draws []FastDraw, limit int) []Ticket {
	tt := NewTopTickets(limit)
	isLotto := config.Name == "lotto"

	for rank := startRank; rank < endRank; rank++ {
		primarySlice, secondarySlice := UnrankTicket(rank, config)
		primaryMask := SliceToMask(primarySlice)
		var secondaryMask uint64
		if len(secondarySlice) > 0 {
			secondaryMask = SliceToMask(secondarySlice)
		}

		var totalPrize int64
		for i := range draws {
			d := &draws[i]
			var matchPrimary, matchSecondary int
			if isLotto {
				// In Lotto, matchPrimary is the match count of primary numbers.
				// We only match matchSecondary (bonus ball) if matchPrimary is exactly 5.
				matchPrimary = bits.OnesCount64(primaryMask & d.PrimaryMask)
				if matchPrimary == 5 {
					matchSecondary = bits.OnesCount64(primaryMask & d.SecondaryMask)
				} else {
					matchSecondary = 0
				}
				totalPrize += d.PrizeMatrix[matchPrimary][matchSecondary]

				if d.IsLottoTwoRound {
					matchPrimary2 := bits.OnesCount64(primaryMask & d.PrimaryMask2)
					var matchSecondary2 int
					if matchPrimary2 == 5 {
						matchSecondary2 = bits.OnesCount64(primaryMask & d.SecondaryMask2)
					} else {
						matchSecondary2 = 0
					}
					totalPrize += d.PrizeMatrix2[matchPrimary2][matchSecondary2]
				}
			} else {
				matchPrimary = bits.OnesCount64(primaryMask & d.PrimaryMask)
				matchSecondary = bits.OnesCount64(secondaryMask & d.SecondaryMask)
				totalPrize += d.PrizeMatrix[matchPrimary][matchSecondary]
			}
		}

		if totalPrize > 0 {
			tt.Add(totalPrize, primarySlice, secondarySlice)
		}
	}

	return tt.Tickets
}
