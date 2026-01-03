# ===== BUILD STAGE =====
FROM golang:1.24.6-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Copy go mod files first for cache
COPY go.mod go.sum ./

# Download dependencies with fresh cache
RUN go mod download

# Copy source code
COPY . .

# Install Orchestrion (latest version)
RUN go install github.com/DataDog/orchestrion@latest

# Ensure all dd-trace-go packages are at latest version
RUN orchestrion pin

# Build with Orchestrion for automatic instrumentation
RUN orchestrion go build -o assessment-service .


# ===== RUNTIME STAGE =====
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/assessment-service .

# Copy migrations if needed
COPY --from=builder /app/migrations ./migrations

# Default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run
ENTRYPOINT ["./assessment-service"]
