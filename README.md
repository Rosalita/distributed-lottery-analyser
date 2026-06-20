# Distributed Lottery Analyser

A highly parallelized, distributed application designed to find the most historically profitable lottery ticket for major UK National Lottery games.

This project is designed to run on top of the custom `distributed-compute-operator` which can be found at https://github.com/Rosalita/distributed-compute-operator.

## Data Gathering

This repository includes a standalone Go script responsible for building and maintaining the historical dataset required for the analysis.

The script is located at `cmd/getdrawhistory/main.go`.

### How it Works

The data gathering process is a two-step pipeline:

1.  **Main CSV Update**: The script first downloads the latest 180-day draw history summary for all four games (Lotto, EuroMillions, Thunderball, and Set For Life). It then intelligently merges this data into a single, de-duplicated `main.csv` file for each game, ensuring a complete and sorted list of all known draws.

2.  **Detailed JSON Fetch**: After updating the main CSVs, the script parses them to extract a list of all historical `DrawNumber`s. It then iterates through these numbers and downloads a detailed JSON file for each individual draw, containing the exact prize breakdown for every tier.

The script is idempotent and efficient. It will automatically skip downloading any CSV or JSON files that it has already successfully fetched, making it safe to run on a regular schedule.

### How to Run

To update the local dataset, run the following command from the root of the repository:

```bash
go run ./cmd/getdrawhistory
```

## License

This project is licensed under the MIT License - see the [LICENSE](file:///c:/dev/go/src/github.com/Rosalita/distributed-lottery-analyser/LICENSE) file for details.