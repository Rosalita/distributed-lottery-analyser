package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

// ExtractDrawNumbers reads a lottery CSV file and extracts all the DrawNumbers.
func ExtractDrawNumbers(filePath string) ([]int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV data from %s: %w", filePath, err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("file %s does not contain enough data", filePath)
	}

	headers := records[0]
	drawNumberIdx := -1
	for i, header := range headers {
		if header == "DrawNumber" {
			drawNumberIdx = i
			break
		}
	}

	if drawNumberIdx == -1 {
		return nil, fmt.Errorf("column 'DrawNumber' not found in %s", filePath)
	}

	var drawNumbers []int
	for _, row := range records[1:] {
		drawNo, err := strconv.Atoi(row[drawNumberIdx])
		if err != nil {
			return nil, fmt.Errorf("invalid draw number '%s': %w", row[drawNumberIdx], err)
		}
		drawNumbers = append(drawNumbers, drawNo)
	}

	return drawNumbers, nil
}