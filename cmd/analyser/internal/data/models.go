package data

import "time"

// DrawDetails represents the structured data for a historical draw.
// This structure is generic and works for EuroMillions, Lotto, Thunderball, and Set For Life.
type DrawDetails struct {
	DrawResult     DrawResult     `json:"drawResult"`
	PrizeBreakdown PrizeBreakdown `json:"prizeBreakdown"`
}

type DrawResult struct {
	GameID       int                 `json:"gameId"`
	DrawNo       int                 `json:"drawNo"`
	DrawDate     time.Time           `json:"drawDate"`
	DrawnNumbers DrawnNumbersWrapper `json:"drawnNumbers"`
}

type DrawnNumbersWrapper struct {
	DrawnNumbers struct {
		PrimaryNumbers   []int `json:"primaryNumbers"`
		SecondaryNumbers []int `json:"secondaryNumbers"`
	} `json:"drawnNumbers"`
}

type PrizeBreakdown struct {
	PrizeLevels []PrizeLevel `json:"prizeLevels"`
}

type PrizeLevel struct {
	MatchLabel         string `json:"matchLabel"`
	MatchBallPrimary   int    `json:"matchBallPrimary"`
	MatchBallSecondary int    `json:"matchBallSecondary"`
	Prize              struct {
		PrizeCents int64 `json:"prizeCents"`
	} `json:"prize"`
}

// CalculateMatches compares a played ticket against the drawn numbers and returns the number of primary and secondary matches.
func (d *DrawDetails) CalculateMatches(playedPrimary, playedSecondary []int) (matchPrimary, matchSecondary int) {
	for _, played := range playedPrimary {
		for _, drawn := range d.DrawResult.DrawnNumbers.DrawnNumbers.PrimaryNumbers {
			if played == drawn {
				matchPrimary++
				break
			}
		}
	}

	for _, played := range playedSecondary {
		for _, drawn := range d.DrawResult.DrawnNumbers.DrawnNumbers.SecondaryNumbers {
			if played == drawn {
				matchSecondary++
				break
			}
		}
	}
	return matchPrimary, matchSecondary
}

// GetPrize returns the prize amount in cents for a given number of primary and secondary matches.
func (d *DrawDetails) GetPrize(matchPrimary, matchSecondary int) int64 {
	for _, level := range d.PrizeBreakdown.PrizeLevels {
		if level.MatchBallPrimary == matchPrimary && level.MatchBallSecondary == matchSecondary {
			return level.Prize.PrizeCents
		}
	}
	return 0
}