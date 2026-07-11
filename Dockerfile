# ==========================================
# STAGE 1: Builder
# ==========================================
# We use the official Go Alpine image matching our project's Go version (1.26).
# Alpine is used because it is lightweight.
FROM golang:1.26-alpine AS builder

# Install system compilation dependencies.
# Alpine is minimal, so we add git and build tools just in case we need them (e.g. for CGO or Make).
RUN apk add --no-cache git make build-base

# Set the working directory inside the build container.
WORKDIR /app

# Copy go.mod and go.sum first.
# By copying these files and running go mod download first, Docker caches this layer.
# Future builds will skip dependency downloads unless go.mod or go.sum changes.
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire workspace into the build container.
COPY . .

# Compile the binaries.
# CGO_ENABLED=0 builds a statically-linked binary, which means it doesn't depend on external libc libraries.
# This is crucial for running in lightweight runner environments.
# GOOS=linux target compilation ensures it runs on the Linux environment of the container.
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/analyser ./cmd/analyser
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/getdrawhistory ./cmd/getdrawhistory


# ==========================================
# STAGE 2: Runner
# ==========================================
# We start from a fresh, clean, and tiny alpine image for execution.
FROM alpine:latest

WORKDIR /app

# Copy the compiled binaries from the builder stage.
COPY --from=builder /bin/analyser /app/analyser
COPY --from=builder /bin/getdrawhistory /app/getdrawhistory

# Copy the existing historical dataset from the host system.
# This copies all the draw files you have accumulated in the repository.
COPY cmd/getdrawhistory/data /app/data

# Expose the port used by the Leader gRPC server.
EXPOSE 50051

# Set the entrypoint to the analyser binary.
# At runtime, you can pass arguments to role (leader/worker) and data directories.
ENTRYPOINT ["/app/analyser"]
