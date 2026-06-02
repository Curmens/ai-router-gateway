# Stage 1: Build the Go binary
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile binary for Linux with static linking
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o ai-router ./cmd/router/main.go

# Stage 2: Minimal runtime image (Debian for glibc compatibility with agy binary)
FROM debian:12-slim

# Install certificates, tzdata, and Node.js (required for claude CLI)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata nodejs npm curl \
    && rm -rf /var/lib/apt/lists/*

# Install claude CLI globally
RUN npm install -g @anthropic-ai/claude-code

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/ai-router /app/ai-router
COPY --from=builder /app/configs/config.example.yaml /app/configs/config.yaml

# Expose HTTP port
EXPOSE 8080

# Execute microservice
ENTRYPOINT ["/app/ai-router"]
