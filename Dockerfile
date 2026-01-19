# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fastship ./cmd/fastship

# Runtime stage
FROM alpine:latest

# Install Docker CLI and Docker Compose
RUN apk add --no-cache \
    docker-cli \
    docker-cli-compose \
    ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/fastship .

# Create config directory
RUN mkdir -p /app/config

# Expose port
EXPOSE 9000

# Run as non-root user (but needs docker group access)
# In production, ensure the user is in the docker group on the host
USER root

ENTRYPOINT ["/app/fastship"]
CMD ["--config", "/app/config/config.yaml"]
