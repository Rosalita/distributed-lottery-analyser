package main

import "time"

// DrawDetailsResponse represents the raw JSON response from the National Lottery API.
// This structure is generic and works for EuroMillions, Lotto, Thunderball, and Set For Life.
type DrawDetailsResponse struct {
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
