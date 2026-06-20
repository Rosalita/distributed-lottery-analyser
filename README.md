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

## Brute-Force Solver Engine

To find the most profitable ticket combination, the analyser uses two key mathematical and performance techniques:

### 1. Combinatorial Number System (Combinadics)

We map the massive multidimensional ticket combination space (e.g., matching 5 main numbers and 1 extra ball) into a flat, 1D rank space from `0` to `TotalCombinations - 1`. 

Using the **Combinatorial Number System (Combinadics)**, a worker can instantaneously decode any rank `R` into its exact combination of numbers without storing combinations in memory or maintaining generator state. This allows the leader to distribute work to worker pods in simple, independent range chunks (e.g., `[1,000,000, 2,000,000)`).

### 2. High-Performance Bitmasking

To evaluate combinations as fast as possible against historical draws, we convert ticket combinations and draw results into `uint64` bitmasks:
- Finding matches between a ticket and a draw is done via a single bitwise AND followed by a population count (`math/bits.OnesCount64`), which Go compiles down to the native CPU `POPCNT` assembly instruction.
- Prize values are pre-parsed into a static 2D lookup matrix `[matchPrimary][matchSecondary]` per draw, avoiding map lookups or conditional checks during the inner simulation loop.

This yields an evaluation loop that runs in a few clock cycles per ticket, enabling evaluation of millions of tickets per second per core.

## License

This project is licensed under the MIT License - see the [LICENSE](file:///c:/dev/go/src/github.com/Rosalita/distributed-lottery-analyser/LICENSE) file for details.