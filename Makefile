.PHONY: install dep build test test-integration lint vet fmt generate clean check check-all

# Install system dependencies (Go, golangci-lint)
install:
	@echo "Checking Go installation..."
	@command -v go >/dev/null 2>&1 || { echo "Go is not installed. Install Go 1.25+"; exit 1; }
	@echo "Installing golangci-lint..."
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@echo "Dependencies installed."

# Install/tidy project dependencies
dep:
	go mod tidy

# Build all packages
build:
	go build ./...

# Run unit tests (race detection, -short, skips Docker tests)
test:
	go test -race -short -count=1 ./...

# Run integration tests
test-integration:
	go test -race -count=1 -run Integration ./...

# Run golangci-lint
lint:
	golangci-lint run ./...

# Run go vet
vet:
	go vet ./...

# Format all Go files
fmt:
	gofmt -w .

# Generate code from specs/schemas
generate:
	go generate ./...

# Clean build artifacts
clean:
	go clean ./...
	rm -rf .ai-build/
	rm -f *.out

# build + lint + vet + unit tests
check: build lint vet test

# build + lint + vet + all tests
check-all: build lint vet test test-integration
