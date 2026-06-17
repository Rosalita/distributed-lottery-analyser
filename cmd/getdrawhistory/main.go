package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DrawHistoryDownloader is responsible for downloading lottery data and updating the main CSV files.
type DrawHistoryDownloader struct {
	client *http.Client
}

// NewDrawHistoryDownloader creates a new instance of the data DrawHistoryDownloader.
func NewDrawHistoryDownloader() *DrawHistoryDownloader {
	return &DrawHistoryDownloader{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DownloadLatestDrawHistories retrieves the latest results for all supported games.
func (d *DrawHistoryDownloader) DownloadLatestDrawHistories() error {
	games := []struct {
		name     string
		dir      string
		url      string
		filename string
	}{
		{"EuroMillions", "euromillions", "https://api-dfe.national-lottery.co.uk/draw-game/results/33/download?interval=ONE_EIGHTY", "euromillions.csv"},
		{"Lotto", "lotto", "https://api-dfe.national-lottery.co.uk/draw-game/results/6/download?interval=ONE_EIGHTY", "lotto.csv"},
		{"Thunderball", "thunderball", "https://api-dfe.national-lottery.co.uk/draw-game/results/4/download?interval=ONE_EIGHTY", "thunderball.csv"},
		{"Set For Life", "setforlife", "https://api-dfe.national-lottery.co.uk/draw-game/results/3/download?interval=ONE_EIGHTY", "setforlife.csv"},
	}

	var downloadErrors []string
	for _, g := range games {
		if err := d.mergeGameCSV(g.name, g.dir, g.url, g.filename); err != nil {
			// Don't stop on the first error; collect all errors to report at the end.
			downloadErrors = append(downloadErrors, fmt.Sprintf("failed to download %s: %v", g.name, err))
		}
	}

	if len(downloadErrors) > 0 {
		return fmt.Errorf("one or more downloads failed:\n- %s", strings.Join(downloadErrors, "\n- "))
	}

	return nil
}

// downloadGameCSV downloads the latest CSV data for a game into memory.
func (d *DrawHistoryDownloader) downloadGameCSV(gameName, url string) ([]byte, error) {
	fmt.Printf("Downloading latest %s draw history...\n", gameName)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://www.national-lottery.co.uk")
	req.Header.Set("Referer", "https://www.national-lottery.co.uk/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// mergeGameCSV handles downloading, merging, and saving the main CSV for a game.
func (d *DrawHistoryDownloader) mergeGameCSV(gameName, dirName, url, filename string) error {
	// Define the path for the main CSV file.
	_, currentFile, _, _ := runtime.Caller(0)
	dirPath := filepath.Join(filepath.Dir(currentFile), "data", dirName)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory %s: %w", dirPath, err)
	}
	mainFilePath := filepath.Join(dirPath, filename)

	// Download the latest 180-day CSV data.
	newDataBytes, err := d.downloadGameCSV(gameName, url)
	if err != nil {
		return err
	}

	// Parse the newly downloaded data.
	newDataReader := csv.NewReader(bytes.NewReader(newDataBytes))
	newRecords, err := newDataReader.ReadAll()
	if err != nil || len(newRecords) < 2 {
		return fmt.Errorf("failed to parse new data for %s", gameName)
	}

	// Read the existing main file.
	mainFile, err := os.Open(mainFilePath)
	var existingRecords [][]string
	if os.IsNotExist(err) {
		fmt.Printf("Main file for %s not found, creating a new one.\n", gameName)
		existingRecords = [][]string{newRecords[0]} // Just use the header
	} else if err != nil {
		return fmt.Errorf("failed to open main file %s: %w", mainFilePath, err)
	} else {
		existingDataReader := csv.NewReader(mainFile)
		existingRecords, err = existingDataReader.ReadAll()
		if err != nil {
			mainFile.Close()
			return fmt.Errorf("failed to parse existing data for %s: %w", gameName, err)
		}
		mainFile.Close()
	}

	// Merge records using a map to handle duplicates, with DrawNumber as the key.
	headers := existingRecords[0]
	drawNumberIdx := -1
	for i, h := range headers {
		if h == "DrawNumber" {
			drawNumberIdx = i
			break
		}
	}
	if drawNumberIdx == -1 {
		return fmt.Errorf("could not find 'DrawNumber' column for %s", gameName)
	}

	allDraws := make(map[string][]string)
	// Add existing records to the map.
	for _, rec := range existingRecords[1:] {
		allDraws[rec[drawNumberIdx]] = rec
	}
	// Add new records, overwriting any duplicates.
	for _, rec := range newRecords[1:] {
		allDraws[rec[drawNumberIdx]] = rec
	}

	// Prepare to write back the sorted, merged data.
	var mergedRecords [][]string
	for _, rec := range allDraws {
		mergedRecords = append(mergedRecords, rec)
	}

	// Sort by DrawNumber in descending order.
	sort.Slice(mergedRecords, func(i, j int) bool {
		numI, _ := strconv.Atoi(mergedRecords[i][drawNumberIdx])
		numJ, _ := strconv.Atoi(mergedRecords[j][drawNumberIdx])
		return numI > numJ
	})

	// Prepend the header row.
	finalRecords := append([][]string{headers}, mergedRecords...)

	// Write the final merged data back to the main file.
	outputFile, err := os.Create(mainFilePath)
	if err != nil {
		return fmt.Errorf("failed to create main file for writing: %w", err)
	}
	defer outputFile.Close()

	writer := csv.NewWriter(outputFile)
	if err := writer.WriteAll(finalRecords); err != nil {
		return fmt.Errorf("failed to write to main file: %w", err)
	}

	fmt.Printf("Successfully merged and updated main CSV for %s. Total draws: %d\n", gameName, len(mergedRecords))
	return nil
}

// FetchDrawDetails downloads the detailed results JSON for a specific draw.
func (d *DrawHistoryDownloader) FetchDrawDetails(gameSlug string, gameId int, drawNo int) (bool, error) {
	// Dynamically get the directory of this script to safely build the data path
	_, currentFile, _, _ := runtime.Caller(0)
	dirPath := filepath.Join(filepath.Dir(currentFile), "data", gameSlug, "draw_details")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return false, fmt.Errorf("failed to create data directory %s: %w", dirPath, err)
	}

	fileName := fmt.Sprintf("%s_draw_%d_details.json", gameSlug, drawNo)
	filePath := filepath.Join(dirPath, fileName)

	// Check if the file already exists to avoid overwriting
	if _, err := os.Stat(filePath); err == nil {
		return false, nil
	}

	fmt.Printf("Fetching details for %s draw %d...\n", gameSlug, drawNo)
	url := fmt.Sprintf("https://api-dfe.national-lottery.co.uk/draw-game/results/%d/%d", gameId, drawNo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://www.national-lottery.co.uk")
	req.Header.Set("Referer", "https://www.national-lottery.co.uk/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := d.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return true, err
}

func main() {
	fmt.Println("Starting manual data update for all lottery games...")
	downloader := NewDrawHistoryDownloader()
	if err := downloader.DownloadLatestDrawHistories(); err != nil {
		fmt.Printf("Error updating data: %v\n", err)
	} else {
		fmt.Println("Successfully downloaded the latest 180 days of data for all games.")
	}

	fmt.Println("\nStarting update of individual draw details...")

	games := []struct {
		name     string
		dir      string
		gameId   int
		filename string
	}{
		{"EuroMillions", "euromillions", 33, "euromillions.csv"},
		{"Lotto", "lotto", 6, "lotto.csv"},
		{"Thunderball", "thunderball", 4, "thunderball.csv"},
		{"Set For Life", "setforlife", 3, "setforlife.csv"},
	}

	totalDownloaded := 0
	totalSkipped := 0

	for _, g := range games {
		fmt.Printf("\nProcessing %s...\n", g.name)

		_, currentFile, _, _ := runtime.Caller(0)
		gameDir := filepath.Join(filepath.Dir(currentFile), "data", g.dir)
		mainCSV := filepath.Join(gameDir, g.filename)

		if _, err := os.Stat(mainCSV); os.IsNotExist(err) {
			fmt.Printf("Could not find main CSV file for %s, skipping.\n", g.name)
			continue
		}

		drawNumbers, err := ExtractDrawNumbers(mainCSV)
		if err != nil {
			fmt.Printf("Error extracting draw numbers for %s: %v\n", g.name, err)
			continue
		}

		fmt.Printf("Found %d draws to check for %s.\n", len(drawNumbers), g.name)

		for _, drawNo := range drawNumbers {
			downloaded, err := downloader.FetchDrawDetails(g.dir, g.gameId, drawNo)
			if err != nil {
				fmt.Printf("Error fetching draw details for draw %d: %v\n", drawNo, err)
			} else if downloaded {
				totalDownloaded++
				fmt.Printf(" -> Downloaded details for draw %d\n", drawNo)
				time.Sleep(1 * time.Second) // Be polite to the API
			} else {
				totalSkipped++
			}
		}
	}

	fmt.Printf("\nUpdate complete. Downloaded %d new files, skipped %d existing files.\n", totalDownloaded, totalSkipped)
}
