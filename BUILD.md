# Building FastShip

## Prerequisites

- Go 1.22 or higher

## Installation

### 1. Install Go

**macOS:**
```bash
brew install go
```

**Linux:**
```bash
# Download and install Go 1.22+
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### 2. Download Dependencies

```bash
cd /path/to/fastship
go mod download
```

### 3. Build

```bash
# Build for current platform
make build

# Or manually:
go build -o fastship ./cmd/fastship
```

### 4. Build for Linux (from macOS)

```bash
make build-linux

# Or manually:
GOOS=linux GOARCH=amd64 go build -o fastship-linux-amd64 ./cmd/fastship
```

## Running

### 1. Create Configuration

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your settings
```

### 2. Run FastShip

```bash
./fastship --config config.yaml
```

## Testing

```bash
# Run all tests
make test

# Or manually:
go test -v ./...
```

## Development

### Running in Development Mode

```bash
# Run without building
go run ./cmd/fastship --config config.yaml
```

### Code Structure

```
fastship/
├── cmd/fastship/main.go         # Application entry point
├── internal/
│   ├── config/                  # Configuration parsing
│   ├── webhook/                 # HTTP handlers and auth
│   ├── deploy/                  # Deployment orchestration
│   └── docker/                  # Docker client
├── config.example.yaml          # Example configuration
└── Makefile                     # Build commands
```

### Adding New Features

1. Configuration changes go in `internal/config/config.go`
2. HTTP endpoints go in `internal/webhook/handler.go`
3. Deployment logic goes in `internal/deploy/deploy.go`
4. Docker operations go in `internal/docker/docker.go`

## Deployment

### As a Binary

```bash
# Build for Linux
make build-linux

# Copy to server
scp fastship-linux-amd64 user@server:/usr/local/bin/fastship

# Create systemd service
sudo systemctl enable fastship
sudo systemctl start fastship
```

### As a Docker Container

See README.md for Docker deployment instructions.
