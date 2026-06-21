package data

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// LoadGameData reads draw details from CSV and overlays valid detailed JSON outputs.
func LoadGameData(gameName, baseDataDir string) (map[int]DrawDetails, error) {
	// 1. Load historical draws from CSV as a baseline.
	csvPath := filepath.Join(baseDataDir, gameName, gameName+".csv")
	draws, err := parseCSVDraws(csvPath, gameName)
	if err != nil {
		fmt.Printf("Warning: CSV file %s not found or failed to parse: %v. Fallback to empty baseline.\n", csvPath, err)
		draws = make(map[int]DrawDetails)
	}

	// 2. Overlay successful/detailed JSON draw details.
	dataDir := filepath.Join(baseDataDir, gameName, "draw_details")
	files, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return draws, nil
		}
		return nil, fmt.Errorf("failed to read directory %s: %w", dataDir, err)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(dataDir, file.Name())
		fileBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
		}

		// Verify it's a valid JSON response (doesn't contain error code RESOURCE_NOT_FOUND etc.)
		if bytes.Contains(fileBytes, []byte(`"RESOURCE_NOT_FOUND"`)) || bytes.Contains(fileBytes, []byte(`"VALIDATION_ERROR"`)) {
			continue
		}

		var details DrawDetails
		if err := json.Unmarshal(fileBytes, &details); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON in file %s: %w", filePath, err)
		}

		// Only overlay if the JSON parsed successfully and contains actual draw details
		if details.DrawResult.DrawNo > 0 && len(details.DrawResult.DrawnNumbers.DrawnNumbers.PrimaryNumbers) > 0 {
			// Clean up prize levels: ignore raffle levels like UK Millionaire Maker (matching 0+0)
			var filteredLevels []PrizeLevel
			for _, lvl := range details.PrizeBreakdown.PrizeLevels {
				if lvl.MatchBallPrimary == 0 && lvl.MatchBallSecondary == 0 {
					continue
				}
				filteredLevels = append(filteredLevels, lvl)
			}
			details.PrizeBreakdown.PrizeLevels = filteredLevels

			draws[details.DrawResult.DrawNo] = details
		}
	}

	return draws, nil
}

// LoadAllData loads historical draw data for all specified games.
// Data is returned as a map[game name]map[draw number]DrawDetails.
func LoadAllData(baseDataDir string) (map[string]map[int]DrawDetails, error) {
	games := []string{"euromillions", "lotto", "thunderball", "setforlife"}
	allData := make(map[string]map[int]DrawDetails)

	for _, game := range games {
		draws, err := LoadGameData(game, baseDataDir)
		if err != nil {
			return nil, fmt.Errorf("error loading data for %s: %w", game, err)
		}
		allData[game] = draws
	}

	return allData, nil
}

func parseCSVDraws(csvPath string, gameName string) (map[int]DrawDetails, error) {
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file %s: %w", csvPath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV records from %s: %w", csvPath, err)
	}

	if len(records) < 2 {
		return nil, nil
	}

	headers := records[0]
	drawNoIdx := findColumnIndex(headers, "DrawNumber")
	if drawNoIdx == -1 {
		return nil, fmt.Errorf("DrawNumber column not found in %s", csvPath)
	}

	draws := make(map[int]DrawDetails)
	for _, row := range records[1:] {
		if len(row) <= drawNoIdx {
			continue
		}
		drawNo, err := strconv.Atoi(row[drawNoIdx])
		if err != nil {
			continue
		}

		var primary []int
		var secondary []int

		switch gameName {
		case "lotto":
			primary = getIntSlice(row, headers, []string{"Ball 1", "Ball 2", "Ball 3", "Ball 4", "Ball 5", "Ball 6"})
			secondary = getIntSlice(row, headers, []string{"Bonus Ball"})
		case "euromillions":
			primary = getIntSlice(row, headers, []string{"Ball 1", "Ball 2", "Ball 3", "Ball 4", "Ball 5"})
			secondary = getIntSlice(row, headers, []string{"Lucky Star 1", "Lucky Star 2"})
		case "thunderball":
			primary = getIntSlice(row, headers, []string{"Ball 1", "Ball 2", "Ball 3", "Ball 4", "Ball 5"})
			secondary = getIntSlice(row, headers, []string{"Thunderball"})
		case "setforlife":
			primary = getIntSlice(row, headers, []string{"Ball 1", "Ball 2", "Ball 3", "Ball 4", "Ball 5"})
			secondary = getIntSlice(row, headers, []string{"Life Ball"})
		}

		d := DrawDetails{}
		d.DrawResult.GameID = getGameID(gameName)
		d.DrawResult.DrawNo = drawNo
		d.DrawResult.DrawnNumbers.DrawnNumbers.PrimaryNumbers = primary
		d.DrawResult.DrawnNumbers.DrawnNumbers.SecondaryNumbers = secondary
		d.PrizeBreakdown.PrizeLevels = getDefaultPrizeLevels(gameName)

		draws[drawNo] = d
	}

	return draws, nil
}

func getGameID(gameName string) int {
	switch gameName {
	case "lotto":
		return 6
	case "euromillions":
		return 33
	case "thunderball":
		return 4
	case "setforlife":
		return 3
	}
	return 0
}

func findColumnIndex(headers []string, name string) int {
	for i, h := range headers {
		if h == name {
			return i
		}
	}
	return -1
}

func getIntSlice(row []string, headers []string, names []string) []int {
	var vals []int
	for _, name := range names {
		idx := findColumnIndex(headers, name)
		if idx != -1 && idx < len(row) && row[idx] != "" {
			val, err := strconv.Atoi(row[idx])
			if err == nil {
				vals = append(vals, val)
			}
		}
	}
	return vals
}

func getDefaultPrizeLevels(gameName string) []PrizeLevel {
	var levels []PrizeLevel

	addLevel := func(round string, primary, secondary int, prizePence int64, label string) {
		pl := PrizeLevel{
			DrawRound:          round,
			MatchLabel:         label,
			MatchBallPrimary:   primary,
			MatchBallSecondary: secondary,
		}
		pl.Prize.PrizePence = prizePence
		levels = append(levels, pl)
	}

	switch gameName {
	case "lotto":
		addLevel("ONE", 6, 0, 500000000, "Match 6")
		addLevel("ONE", 5, 1, 100000000, "Match 5 + Bonus")
		addLevel("ONE", 5, 0, 175000, "Match 5")
		addLevel("ONE", 4, 0, 14000, "Match 4")
		addLevel("ONE", 3, 0, 3000, "Match 3")
		addLevel("ONE", 2, 0, 200, "Match 2")
	case "thunderball":
		addLevel("ONE", 5, 1, 50000000, "Match 5 + Thunderball")
		addLevel("ONE", 5, 0, 500000, "Match 5")
		addLevel("ONE", 4, 1, 25000, "Match 4 + Thunderball")
		addLevel("ONE", 4, 0, 10000, "Match 4")
		addLevel("ONE", 3, 1, 2000, "Match 3 + Thunderball")
		addLevel("ONE", 3, 0, 1000, "Match 3")
		addLevel("ONE", 2, 1, 1000, "Match 2 + Thunderball")
		addLevel("ONE", 1, 1, 500, "Match 1 + Thunderball")
		addLevel("ONE", 0, 1, 300, "Match 0 + Thunderball")
	case "setforlife":
		addLevel("ONE", 5, 1, 360000000, "Match 5 + Life Ball")
		addLevel("ONE", 5, 0, 12000000, "Match 5")
		addLevel("ONE", 4, 1, 25000, "Match 4 + Life Ball")
		addLevel("ONE", 4, 0, 5000, "Match 4")
		addLevel("ONE", 3, 1, 3000, "Match 3 + Life Ball")
		addLevel("ONE", 3, 0, 2000, "Match 3")
		addLevel("ONE", 2, 1, 1000, "Match 2 + Life Ball")
		addLevel("ONE", 2, 0, 500, "Match 2")
	case "euromillions":
		addLevel("ONE", 5, 2, 5000000000, "Match 5 + 2 Lucky Stars")
		addLevel("ONE", 5, 1, 20000000, "Match 5 + 1 Lucky Star")
		addLevel("ONE", 5, 0, 2000000, "Match 5")
		addLevel("ONE", 4, 2, 150000, "Match 4 + 2 Lucky Stars")
		addLevel("ONE", 4, 1, 12000, "Match 4 + 1 Lucky Star")
		addLevel("ONE", 3, 2, 9000, "Match 3 + 2 Lucky Stars")
		addLevel("ONE", 4, 0, 4000, "Match 4")
		addLevel("ONE", 2, 2, 1400, "Match 2 + 2 Lucky Stars")
		addLevel("ONE", 3, 1, 1100, "Match 3 + 1 Lucky Star")
		addLevel("ONE", 3, 0, 900, "Match 3")
		addLevel("ONE", 1, 2, 700, "Match 1 + 2 Lucky Stars")
		addLevel("ONE", 2, 1, 600, "Match 2 + 1 Lucky Star")
		addLevel("ONE", 2, 0, 400, "Match 2")
	}

	return levels
}