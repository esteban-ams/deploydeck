# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod file and download dependencies
COPY go.mod ./
RUN go mod download || true

# Copy source code and generate go.sum
COPY . .
RUN go mod tidy

# Build the binary
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-X main.version=${VERSION}" -o deploydeck ./cmd/deploydeck

# Runtime stage
FROM alpine:latest

# Install Docker CLI and Docker Compose
RUN apk add --no-cache \
    docker-cli \
    docker-cli-compose \
    git \
    ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/deploydeck .

# Expose port
EXPOSE 9000

# Run as non-root user (but needs docker group access)
# In production, ensure the user is in the docker group on the host
USER root

ENTRYPOINT ["/app/deploydeck"]
CMD ["--config", "/app/config.yaml"]
