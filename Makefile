.PHONY: proto test build clean

proto:
	protoc --go_out=. --go_opt=module=github.com/Rosalita/distributed-lottery-analyser --go-grpc_out=. --go-grpc_opt=module=github.com/Rosalita/distributed-lottery-analyser protos/analyser.proto

test:
	go test -v ./...

build:
	go build -o bin/analyser.exe ./cmd/analyser

clean:
	rm -rf protos/generated/analyser/*.go
