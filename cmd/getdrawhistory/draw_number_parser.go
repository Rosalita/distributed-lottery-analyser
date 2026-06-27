package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DrawInfo stores the draw number and the date it was drawn.
type DrawInfo struct {
	DrawNumber int
	DrawDate   time.Time
}

// ExtractDrawInfos reads a lottery CSV file and extracts DrawInfos containing DrawNumbers and DrawDates.
func ExtractDrawInfos(filePath string) ([]DrawInfo, error) {
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
	drawDateIdx := -1
	for i, header := range headers {
		// Clean header: trim UTF-8 BOM, spaces, and compare case-insensitively
		cleanHeader := strings.TrimPrefix(header, "\xef\xbb\xbf")
		cleanHeader = strings.TrimPrefix(cleanHeader, "\ufeff")
		cleanHeader = strings.TrimSpace(cleanHeader)
		cleanHeader = strings.ToLower(cleanHeader)

		if cleanHeader == "drawnumber" {
			drawNumberIdx = i
		} else if cleanHeader == "drawdate" {
			drawDateIdx = i
		}
	}

	if drawNumberIdx == -1 {
		return nil, fmt.Errorf("column 'DrawNumber' not found in %s", filePath)
	}
	if drawDateIdx == -1 {
		return nil, fmt.Errorf("column 'DrawDate' not found in %s", filePath)
	}

	var drawInfos []DrawInfo
	for _, row := range records[1:] {
		drawNo, err := strconv.Atoi(row[drawNumberIdx])
		if err != nil {
			return nil, fmt.Errorf("invalid draw number '%s': %w", row[drawNumberIdx], err)
		}

		drawDate, err := time.Parse("02-Jan-2006", row[drawDateIdx])
		if err != nil {
			return nil, fmt.Errorf("invalid draw date '%s': %w", row[drawDateIdx], err)
		}

		drawInfos = append(drawInfos, DrawInfo{
			DrawNumber: drawNo,
			DrawDate:   drawDate,
		})
	}

	return drawInfos, nil
}