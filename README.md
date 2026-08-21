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

Options:
- `--data-dir`: Custom base directory to download historical draw data into. If not provided, falls back to source-relative `data` path.

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
- `--data-dir`: Base directory containing historical draw data. If not provided, falls back to source-relative path.

#### 3. Start a Worker Client
In a new terminal window, start a worker process to connect and execute chunk tasks:
```bash
go run ./cmd/analyser --role=worker --leader=localhost:50051
```
Worker options:
- `--leader`: Address of the leader coordinator. Default is `localhost:50051`.

Once all chunk ranges have been evaluated by workers, the worker process will exit gracefully. The leader will output the top tickets and their historical payouts, then shut down.

### Running with Docker & Docker Compose

A multi-stage `Dockerfile` and a `docker-compose.yml` configuration are provided to run the distributed leader-worker architecture containerized.

#### 1. Build and Run the Leader & Worker

To start both the Leader gRPC server and a Worker client automatically, execute the following from the root of the repository. The first time will need --build and might take a while as it's building the image from scratch:
```bash
docker compose up --build
```

The Leader will boot up and load historical draws from the host's directory (via a bind mount). The Worker will then connect, request ranges to evaluate, and compute them. Once finished, the Leader outputs the results and terminates.

#### 2. Scaling Worker Count

Since workers are stateless, you can scale them horizontally to speed up evaluations. Start the setup with multiple worker containers:
```bash
docker compose up --scale worker=3
```
This boots 1 Leader and 3 parallel Workers.

#### 3. Manual Container Execution

You can also run the built Docker images manually:

* **Build the Docker Image**:
  ```bash
  docker build -t lottery-analyser:latest .
  ```
* **Run Leader Coordinator**:
  ```bash
  docker run -p 50051:50051 -v ./cmd/getdrawhistory/data:/app/data lottery-analyser:latest --role=leader --game=thunderball --data-dir=/app/data
  ```
* **Run Worker Client**:
  ```bash
  docker run --network="host" lottery-analyser:latest --role=worker --leader=localhost:50051
  ```

---

### Kubernetes Deployment with Custom Resource Definition (`DistributedJob`)

This application is designed to be orchestrated natively in Kubernetes using the custom [`distributed-compute-operator`](https://github.com/Rosalita/distributed-compute-operator) (`DistributedJob` Custom Resource).

When a `DistributedJob` resource is created:
1. The operator automatically provisions a **Headless Service** (`<job-name>-svc`) enabling direct pod-to-pod DNS without load-balancing.
2. The operator creates a **Leader Pod** (`<job-name>-leader`) which loads historical lottery data into memory and exposes the gRPC coordinator service.
3. The operator spins up $N$ **Worker Pods** (`<job-name>-worker-0`, `<job-name>-worker-1`, ...) which automatically discover the leader via `<job-name>-leader.<job-name>-svc:50051`, evaluate combination chunks in parallel, and report top tickets back to the leader.

#### 1. Start a Local Kubernetes Cluster with Docker

You can use any local Kubernetes cluster backed by Docker:

* **Option A: Docker Desktop (Recommended on Windows/macOS)**:
  1. Open **Docker Desktop Settings** -> **Kubernetes**.
  2. Check **Enable Kubernetes** and click **Apply & Restart**.
  3. Ensure your context is set: `kubectl config use-context docker-desktop`

* **Option B: KinD (Kubernetes in Docker)**:
  ```bash
  kind create cluster --name lottery-cluster
  ```

* **Option C: Minikube**:
  ```bash
  minikube start --driver=docker
  ```

#### 2. Build and Load the Container Image

Build the `lottery-analyser` Docker image locally:
```bash
docker build -t lottery-analyser:latest .
```

If you are using **KinD** or **Minikube**, load the locally-built image into the cluster nodes:
```bash
# For KinD:
kind load docker-image lottery-analyser:latest --name lottery-cluster

# For Minikube:
minikube image load lottery-analyser:latest
```
*(Note: If using Docker Desktop Kubernetes, local Docker images are already directly available to the cluster).*

#### 3. Install the Operator CRD & Start the Controller Manager

The `DistributedJob` custom resource is orchestrated by the [`distributed-compute-operator`](https://github.com/Rosalita/distributed-compute-operator). The operator controller manager must be actively running to reconcile `DistributedJob` manifests and automatically provision the Leader Pod, Worker Pods, and Headless Service.

From your local clone of [`distributed-compute-operator`](https://github.com/Rosalita/distributed-compute-operator):

1. **Install the Custom Resource Definition (CRD)**:
   ```bash
   kubectl apply -k config/crd
   ```
   Verify CRD installation:
   ```bash
   kubectl get crds
   # You should see: distributedjobs.hpc.rosalita.github.io
   ```

2. **Start the Controller Manager**:
   
   - **Option A (Local Development - Recommended for testing)**: Keep the controller running in a **separate terminal window**:
     ```bash
     go run ./cmd/main.go
     ```
   
   - **Option B (In-Cluster Deployment - Production style)**: Build and deploy the operator directly into your Kubernetes cluster:
     ```bash
     make deploy
     ```

> [!IMPORTANT]
> The controller manager must remain running while jobs are active. If the controller is not running when you apply a `DistributedJob`, Kubernetes will store the resource definition but will not create any Pods or Services.

#### 4. Deploy a `DistributedJob`

Deploy one of the ready-to-use sample manifests from the `deploy/` directory:

```bash
# Deploy Thunderball Analysis (4 workers)
kubectl apply -f deploy/distributedjob-thunderball.yaml

# Or Lotto (4 workers)
kubectl apply -f deploy/distributedjob-lotto.yaml

# Or Set For Life (4 workers)
kubectl apply -f deploy/distributedjob-setforlife.yaml

# Or EuroMillions (8 workers)
kubectl apply -f deploy/distributedjob-euromillions.yaml
```

#### 5. Monitor Job Progress & View Results

Check the custom resource status:
```bash
kubectl get distributedjobs
```
Output:
```text
NAME                  PHASE     WORKERS   ACTIVE   AGE
lottery-thunderball   Running   4         4        15s
```

Check the pods and headless service created by the operator:
```bash
kubectl get pods
kubectl get svc
```

Stream the Leader logs in real time to monitor progress percentages and view the final top winning combinations:
```bash
kubectl logs -f lottery-thunderball-leader -c compute
```

Example output:
```text
[Progress] 4 of 5 chunks completed (80.00%)
[Progress] 5 of 5 chunks completed (100.00%)
All chunks completed successfully!

==================================================
TOP 5 MOST PROFITABLE TICKETS FOR THUNDERBALL
==================================================
1. Primary: [12 18 24 31 38], Secondary: [9] | Total Earnings: £500,240.00
2. Primary: [4 11 19 28 35], Secondary: [3] | Total Earnings: £500,180.00
...
==================================================
```

#### 6. Clean Up

Deleting the `DistributedJob` automatically tears down the leader pod, all worker pods, and the headless service:
```bash
kubectl delete -f deploy/distributedjob-thunderball.yaml
```

---

### Benchmarking & Profiling

To measure execution performance and analyze bottlenecks under load:

1. **Run Benchmarks**:
   To run the benchmark suite and measure CPU times and memory allocation statistics (`B/op` and `allocs/op`):
   ```bash
   go test -bench=Benchmark -benchmem ./cmd/analyser/internal/evaluator
   ```

   * **First column** (`BenchmarkEvaluateRange_...-16`): The benchmark name. The `-16` indicates the number of CPU threads used (`GOMAXPROCS`).
   * **Second column** (e.g., `493`): The iteration count (`N`). This is the number of times the benchmark loop was executed within the default time limit (1 second).
   * **Third column** (e.g., `2426196 ns/op`): The average execution time per iteration in nanoseconds.
   * **Fourth column** (e.g., `561418 B/op`): The average amount of heap memory allocated per iteration in bytes (`B/op`).
   * **Fifth column** (e.g., `20039 allocs/op`): The average number of heap allocations per iteration.

2. **Generate Performance Profiles**:
   To capture CPU and memory profiles for deep inspection:
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

## Optimisation Strategy and Results

The initial benchmark results for this project were as follows:

```text
BenchmarkEvaluateRange_SetForLife-16                 493           2426196 ns/op          561418 B/op      20039 allocs/op
BenchmarkEvaluateRange_Lotto-16                      538           2237489 ns/op          481169 B/op      10018 allocs/op
BenchmarkEvaluateRange_Thunderball-16                493           2424654 ns/op          560914 B/op      20021 allocs/op
BenchmarkEvaluateRange_EuroMillions-16               478           2451271 ns/op          640672 B/op      20011 allocs/op
```

Pprof showed that the `UnrankCombination` function in `combinadics.go` was using 1.43GB memory for
```golang
combination := make([]int, k)
```
A new function `UnrankCombinationToMask` was created and this function intentionally avoided allocating the memory to create a slice, instead favouring to generate the required bitmask as output. Tests were added to verify that the bitmask created was the same for both functions.

After switching from `UnrankTicket` to `UnrankTicketToMasks` and changing the way that winning tickets were handled (only generating the slice of ticket numbers when those tickets qualified for the leaderboard), the benchmarks were run again.

```text
BenchmarkEvaluateRange_SetForLife-16                 793           1479560 ns/op            2480 B/op         77 allocs/op
BenchmarkEvaluateRange_Lotto-16                      750           1597748 ns/op            1984 B/op         35 allocs/op
BenchmarkEvaluateRange_Thunderball-16                807           1462175 ns/op            1472 B/op         41 allocs/op
BenchmarkEvaluateRange_EuroMillions-16               801           1468501 ns/op             992 B/op         21 allocs/op
```
Looking again at pprof, by only generating slices for winning tickets that qualify for the leaderboard, the memory used for slice allocation was reduced from 1.43GB to 89.50MB.

Looking at how this single change affected the "set for life" draw
* Allocations dropped from 20,039 allocs/op to 77 allocs/op, a reduction of 99.6%
* Memory per draw dropped from 561,418 B/op to 2,480 B/op, a reduction of 99.5%
* Execution speed improved from 2.42 ms/op to 1.47 ms/op, an improvement of 39%

## License

This project is licensed under the MIT License - see the [LICENSE](file:///c:/dev/go/src/github.com/Rosalita/distributed-lottery-analyser/LICENSE) file for details.