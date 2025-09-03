.PHONY: build test clean lint examples run-basic run-dynamic run-server

# Variables
GO=go
GOFLAGS=-trimpath
BINARY_NAME=ratelimit
BUILD_DIR=build
EXAMPLES_DIR=cmd/examples
SERVER_DIR=cmd/server
PKG_DIR=pkg

# Build the main binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

# Run tests
test:
	$(GO) test -v ./...

# Run tests with coverage
test-cover:
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out

# Run linting
lint:
	@echo "Linting..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Installing..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin; \
		golangci-lint run ./...; \
	fi

# Format code
fmt:
	$(GO) fmt ./...

# Verify dependencies
verify:
	$(GO) mod verify

# Tidy dependencies
tidy:
	$(GO) mod tidy

# Build examples
examples: build-basic build-dynamic build-server

# Build basic example
build-basic:
	@echo "Building basic example..."
	@mkdir -p $(BUILD_DIR)/examples
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/examples/basic ./$(EXAMPLES_DIR)/basic

# Build dynamic example
build-dynamic:
	@echo "Building dynamic example..."
	@mkdir -p $(BUILD_DIR)/examples
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/examples/dynamic ./$(EXAMPLES_DIR)/dynamic

# Build server
build-server:
	@echo "Building server..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/server ./$(SERVER_DIR)

# Run basic example
run-basic:
	@echo "Running basic example..."
	$(GO) run $(EXAMPLES_DIR)/basic/main.go

# Run dynamic example
run-dynamic:
	@echo "Running dynamic example..."
	$(GO) run $(EXAMPLES_DIR)/dynamic/main.go

# Run server
run-server:
	@echo "Running server..."
	$(GO) run $(SERVER_DIR)/main.go

# Default target
all: clean build test lint examples

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the main binary"
	@echo "  test         - Run tests"
	@echo "  test-cover   - Run tests with coverage"
	@echo "  clean        - Clean build artifacts"
	@echo "  lint         - Run linter"
	@echo "  fmt          - Format code"
	@echo "  verify       - Verify dependencies"
	@echo "  tidy         - Tidy dependencies"
	@echo "  examples     - Build all examples"
	@echo "  build-basic  - Build basic example"
	@echo "  build-dynamic- Build dynamic example"
	@echo "  build-server - Build server"
	@echo "  run-basic    - Run basic example"
	@echo "  run-dynamic  - Run dynamic example"
	@echo "  run-server   - Run server"
	@echo "  all          - Clean, build, test, lint, and build examples"
	@echo "  help         - Show this help message"
