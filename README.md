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

### 3. Distributed Leader-Worker Architecture

The brute-force engine is built as a highly scaleable coordinator-worker system using gRPC:

- **Leader Coordinator**:
  - Loads historical lottery draw configurations and datasets into memory at startup.
  - Partitions the massive combinadic search space into range chunks (tasks).
  - Distributes tasks to connecting worker clients and monitors task heartbeat activity.
  - Re-allocates chunks if a worker disconnects or times out before reporting back.
  - Aggregates and thread-safely merges reported tickets into a global top-performing tickets leaderboard.

- **Worker Client**:
  - Registers with the leader and caches the game parameters and winning draw history locally.
  - Pre-compiles draw details to CPU-friendly mask structures once at startup.
  - Requests work range chunks in a loop, unranking each combination on-the-fly and running high-speed matching checks.
  - Returns the top profitable combinations back to the leader.
  - Remains completely stateless (needs no persistent disk volume mounts or DB connections).

## Development & Tooling

A `Makefile` is provided to simplify common development tasks:

- **Compile Protobufs**: Generates the Go gRPC code under `protos/generated/analyser/` from the schema:
  ```bash
  make proto
  ```
- **Run Unit Tests**: Runs all unit test suites (combinadics, evaluator, coordinator, worker):
  ```bash
  make test
  ```
- **Build Binary**: Compiles the solver application executable to `bin/analyser.exe`:
  ```bash
  make build
  ```
- **Clean**: Cleans up all generated protobuf files:
  ```bash
  make clean
  ```

### Benchmarking & Profiling

To measure execution performance and analyze bottlenecks under load:

1. **Run Benchmarks**:
   To run the benchmark suite and measure CPU times and memory allocation statistics (`B/op` and `allocs/op`):
   ```bash
   go test -bench=Benchmark -benchmem ./cmd/analyser/internal/evaluator
   ```
   The first time benchmarks were run for this project, they looked like this:
   ```text
   BenchmarkEvaluateRange_SetForLife-16                 493           2426196 ns/op          561418 B/op      20039 allocs/op
   BenchmarkEvaluateRange_Lotto-16                      538           2237489 ns/op          481169 B/op      10018 allocs/op
   BenchmarkEvaluateRange_Thunderball-16                493           2424654 ns/op          560914 B/op      20021 allocs/op
   BenchmarkEvaluateRange_EuroMillions-16               478           2451271 ns/op          640672 B/op      20011 allocs/op
   ```
   * **First column** (`BenchmarkEvaluateRange_...-16`): The benchmark name. The `-16` indicates the number of CPU threads used (`GOMAXPROCS`).
   * **Second column** (e.g., `493`): The iteration count (`N`). This is the number of times the benchmark loop was executed within the default time limit (1 second).
   * **Third column** (e.g., `2426196 ns/op`): The average execution time per iteration in nanoseconds.
   * **Fourth column** (e.g., `561418 B/op`): The average amount of heap memory allocated per iteration in bytes (`B/op`).
   * **Fifth column** (e.g., `20039 allocs/op`): The average number of heap allocations per iteration.

2. **Generate Performance Profiles**:
   To capture CPU and memory profiles for deep inspection
   ```bash
  go test -bench=Benchmark -benchmem -cpuprofile=cpu.pprof -memprofile=mem.pprof ./cmd/analyser/internal/evaluator
   ```
   
  Note, if using windows powershell you may need to use space separated arguments as shown below:
   ```bash
   go test -bench=Benchmark -benchmem -cpuprofile cpu.pprof -memprofile mem.pprof ./cmd/analyser/internal/evaluator
   ```

   This will create files named cpu.pprof and mem.pprof in the current directory. A file called evaluator.test.exe will also be created.

3. **Analyze CPU Profiling Hotspots**:
   To view the most CPU-intensive functions:
   ```bash
   go tool pprof -top cpu.pprof
   ```

4. **Analyze Memory Profiling Data (Allocations)**:
   To view heap memory allocation statistics and see where allocations are happening:
   * **Allocated Space** (total bytes allocated, including GC'd memory - best for finding GC overhead):
     ```bash
     go tool pprof -alloc_space -top mem.pprof
     ```
   * **Allocated Objects** (total number of objects created):
     ```bash
     go tool pprof -alloc_objects -top mem.pprof
     ```
   * **In-Use Space** (memory currently retained on the heap - best for finding memory leaks):
     ```bash
     go tool pprof -inuse_space -top mem.pprof
     ```

5. **Launch the Interactive Web UI**:
   To view flame graphs, source code annotations, and visual call graphs in your web browser:
   ```bash
   go tool pprof -http=:8080 mem.pprof
   ```
   *(Navigate to `http://localhost:8080` in your browser).*

### Running the Solver Locally

To run the solver engine on your local machine, follow these steps:

#### 1. Compile Protobuf and Verify Tests
If you make changes to the protobuf schema, compile them using the provided `Makefile`:
```bash
make proto
```
Run the test suites:
```bash
make test
```

#### 2. Start the Leader Coordinator
The leader loads the historical draws dataset and starts a gRPC coordinator server. 

Here are the commands to start the leader for each supported game, along with recommended chunk sizes matching their total combination spaces:

```bash
# Thunderball (8,060,598 total combinations)
go run ./cmd/analyser --role=leader --game=thunderball --chunk-size=2000000 --limit=5

# Lotto (45,057,474 total combinations)
go run ./cmd/analyser --role=leader --game=lotto --chunk-size=5000000 --limit=5

# Set For Life (15,339,390 total combinations)
go run ./cmd/analyser --role=leader --game=setforlife --chunk-size=2000000 --limit=5

# EuroMillions (139,838,160 total combinations)
go run ./cmd/analyser --role=leader --game=euromillions --chunk-size=10000000 --limit=5
```
Leader options:
- `--game`: The lottery game data to analyze (`thunderball`, `lotto`, `euromillions`, `setforlife`). Default is `thunderball`.
- `--chunk-size`: Size of combinadic range chunks distributed to workers. Default is `100,000`.
- `--limit`: Number of top ticket combinations to compile. Default is `5`.
- `--port`: The port to run the gRPC server on. Default is `50051`.

#### 3. Start a Worker Client
In a new terminal window, start a worker process to connect and execute chunk tasks:
```bash
go run ./cmd/analyser --role=worker --leader=localhost:50051
```
Worker options:
- `--leader`: Address of the leader coordinator. Default is `localhost:50051`.

Once all chunk ranges have been evaluated by workers, the worker process will exit gracefully. The leader will output the top tickets and their historical payouts, then shut down.

## License

This project is licensed under the MIT License - see the [LICENSE](file:///c:/dev/go/src/github.com/Rosalita/distributed-lottery-analyser/LICENSE) file for details.