# Implementation Task Checklist

## Stage 1: Combinadics & Evaluator Core
- [x] Implement combination ranking/unranking logic (`internal/evaluator/combinadics.go`)
- [x] Implement high-performance bitmask-based evaluator (`internal/evaluator/evaluator.go`)
- [x] Write unit tests for combinadics and evaluator (`internal/evaluator/evaluator_test.go`)
- [x] Verify tests pass successfully

## Stage 2: gRPC Setup & Code Generation
- [x] Add gRPC and protobuf dependencies to `go.mod`
- [x] Write the gRPC protobuf service definition (`protos/analyser.proto`)
- [x] Compile protobuf to Go files using `protoc`

## Stage 3: Leader Coordinator Server
- [x] Implement Leader coordinator logic (`cmd/analyser/internal/coordinator/leader.go`)
- [x] Integrate draw history loading into the coordinator

## Stage 4: Worker Client
- [x] Implement Worker client execution loop (`cmd/analyser/internal/worker/worker.go`)

## Stage 5: Main Integration & Verification
- [x] Update `cmd/analyser/main.go` with CLI flag parsing and orchestration entrypoints
- [x] Verify end-to-end local execution of leader + worker for a game (e.g., Thunderball)

## Stage 6: Containerization & Deployment
- [ ] Write the `Dockerfile` for containerizing the application
- [ ] Create the Kubernetes `DistributedJob` custom resource YAML to deploy the solver to the Kubernetes cluster
