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
	TopPrize     struct {
		PrizePence int64 `json:"prizeCents"`
	} `json:"topPrize"`
}

type DrawnNumbersWrapper struct {
	DrawnNumbers struct {
		PrimaryNumbers   []int `json:"primaryNumbers"`
		SecondaryNumbers []int `json:"secondaryNumbers"`
	} `json:"drawnNumbers"`
	DrawnNumbersAdditional *struct {
		PrimaryNumbers   []int `json:"primaryNumbers"`
		SecondaryNumbers []int `json:"secondaryNumbers"`
	} `json:"drawnNumbersAdditional"`
}

type PrizeBreakdown struct {
	PrizeLevels []PrizeLevel `json:"prizeLevels"`
}

type PrizeLevel struct {
	DrawRound          string `json:"drawRound"`
	MatchLabel         string `json:"matchLabel"`
	MatchBallPrimary   int    `json:"matchBallPrimary"`
	MatchBallSecondary int    `json:"matchBallSecondary"`
	Prize              struct {
		PrizePence int64 `json:"prizeCents"`
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

// GetPrize returns the prize amount in pence for a given number of primary and secondary matches.
func (d *DrawDetails) GetPrize(matchPrimary, matchSecondary int) int64 {
	return d.GetPrizeForRound(matchPrimary, matchSecondary, "ONE")
}

// GetPrizeForRound returns the prize amount in pence for a given round ("ONE" or "TWO").
func (d *DrawDetails) GetPrizeForRound(matchPrimary, matchSecondary int, round string) int64 {
	for _, level := range d.PrizeBreakdown.PrizeLevels {
		// Matches target round or default round ONE if DrawRound is empty.
		matchRound := level.DrawRound == round || (round == "ONE" && (level.DrawRound == "" || level.DrawRound == "ONE"))
		if matchRound && level.MatchBallPrimary == matchPrimary && level.MatchBallSecondary == matchSecondary {
			prize := level.Prize.PrizePence
			// If it's a jackpot tier and the prize is 0 due to 0 winners, override with top prize jackpot.
			if prize == 0 && d.isJackpotTier(matchPrimary, matchSecondary) {
				prize = d.DrawResult.TopPrize.PrizePence
			}
			return prize
		}
	}
	return 0
}

func (d *DrawDetails) isJackpotTier(matchPrimary, matchSecondary int) bool {
	switch d.DrawResult.GameID {
	case 6, 1: // Lotto
		return matchPrimary == 6 && matchSecondary == 0
	case 33: // EuroMillions
		return matchPrimary == 5 && matchSecondary == 2
	case 4: // Thunderball
		return matchPrimary == 5 && matchSecondary == 1
	case 3: // Set For Life
		return matchPrimary == 5 && matchSecondary == 1
	}
	return false
}