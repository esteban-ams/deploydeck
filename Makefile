.PHONY: build build-linux run test clean install deps

VERSION ?= dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

# Build the binary
build:
	go build $(LDFLAGS) -o fastship ./cmd/fastship

# Build for Linux (useful for deployment from macOS)
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o fastship-linux-amd64 ./cmd/fastship

# Run the application
run:
	go run ./cmd/fastship --config config.yaml

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f fastship fastship-linux-amd64

# Install dependencies
deps:
	go mod download
	go mod tidy

# Install the binary to GOPATH/bin
install:
	go install $(LDFLAGS) ./cmd/fastship
