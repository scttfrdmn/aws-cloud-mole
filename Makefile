.PHONY: build test lint release clean install dev fmt vet security

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS = -ldflags "-s -w \
	-X github.com/research-computing/mole/internal/version.Version=$(VERSION) \
	-X github.com/research-computing/mole/internal/version.Commit=$(COMMIT) \
	-X github.com/research-computing/mole/internal/version.Date=$(DATE)"

# Build targets
build:
	@echo "Building mole $(VERSION)..."
	@go build $(LDFLAGS) -o bin/mole ./cmd/mole

build-all:
	@echo "Building for all platforms..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/mole-linux-amd64 ./cmd/mole
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/mole-linux-arm64 ./cmd/mole
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/mole-darwin-amd64 ./cmd/mole
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/mole-darwin-arm64 ./cmd/mole
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/mole-windows-amd64.exe ./cmd/mole

install: build
	@echo "Installing mole to $(GOPATH)/bin..."
	@install bin/mole $(GOPATH)/bin/mole

# Development targets
dev:
	@echo "Running in development mode..."
	@go run $(LDFLAGS) ./cmd/mole

run: build
	@./bin/mole

# Testing and quality
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

test-short:
	@go test -short -v ./...

bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Code quality
fmt:
	@echo "Formatting code..."
	@go fmt ./...

vet:
	@echo "Running go vet..."
	@go vet ./...

lint:
	@echo "Running golangci-lint..."
	@golangci-lint run

security:
	@echo "Running security scan..."
	@gosec ./...

# Dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download

tidy:
	@echo "Tidying dependencies..."
	@go mod tidy

# Release targets
release:
	@echo "Creating release..."
	@goreleaser release --clean

snapshot:
	@echo "Creating snapshot release..."
	@goreleaser release --snapshot --clean

# Docker targets
docker-build:
	@echo "Building Docker image..."
	@docker build -t mole:latest -t mole:$(VERSION) .

docker-run:
	@docker run --rm -it mole:latest

# Cleanup
clean:
	@echo "Cleaning up..."
	@rm -rf bin/ dist/ coverage.out coverage.html

# Documentation
docs:
	@echo "Generating documentation..."
	@go run ./cmd/mole completion bash > docs/completion.bash
	@go run ./cmd/mole completion zsh > docs/completion.zsh
	@go run ./cmd/mole completion fish > docs/completion.fish

# Help target
help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  build-all   - Build for all supported platforms"
	@echo "  install     - Install binary to GOPATH/bin"
	@echo "  dev         - Run in development mode"
	@echo "  test        - Run all tests with coverage"
	@echo "  test-short  - Run short tests only"
	@echo "  bench       - Run benchmarks"
	@echo "  fmt         - Format code"
	@echo "  vet         - Run go vet"
	@echo "  lint        - Run linter"
	@echo "  security    - Run security scanner"
	@echo "  deps        - Download dependencies"
	@echo "  tidy        - Tidy dependencies"
	@echo "  release     - Create release with goreleaser"
	@echo "  snapshot    - Create snapshot release"
	@echo "  clean       - Clean build artifacts"
	@echo "  help        - Show this help"