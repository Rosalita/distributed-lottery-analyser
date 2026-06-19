package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadGameData reads all JSON draw details for a given game from the specified directory.
func LoadGameData(dataDir string) (map[int]DrawDetails, error) {
	draws := make(map[int]DrawDetails)

	files, err := os.ReadDir(dataDir)
	if err != nil {
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

		var details DrawDetails
		if err := json.Unmarshal(fileBytes, &details); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON in file %s: %w", filePath, err)
		}

		draws[details.DrawResult.DrawNo] = details
	}

	return draws, nil
}

// LoadAllData loads historical draw data for all specified games.
// Data is returned as a map[game name]map[draw number]DrawDetails.
func LoadAllData(baseDataDir string) (map[string]map[int]DrawDetails, error) {
	games := []string{"euromillions", "lotto", "thunderball", "setforlife"}
	allData := make(map[string]map[int]DrawDetails)

	for _, game := range games {
		gameDir := filepath.Join(baseDataDir, game, "draw_details")
		if _, err := os.Stat(gameDir); os.IsNotExist(err) {
			fmt.Printf("Warning: data directory for %s does not exist, skipping.\n", game)
			continue
		}

		draws, err := LoadGameData(gameDir)
		if err != nil {
			return nil, fmt.Errorf("error loading data for %s: %w", game, err)
		}
		allData[game] = draws
	}

	return allData, nil
}