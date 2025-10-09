# Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


.PHONY: build test clean lint examples run-basic run-dynamic run-server run-resilient run-dynamic-resilient run-dynamic-only build-grpc-server run-grpc-server

# Variables
GO=go
GOFLAGS=-trimpath
BINARY_NAME=ratelimit
BUILD_DIR=build
EXAMPLES_DIR=cmd/examples
SERVER_DIR=examples/server
PKG_DIR=pkg
RESILIENT_DIR=examples/resilient_limiter
DYNAMIC_RESILIENT_DIR=examples/dynamic_resilient
DYNAMIC_ONLY_DIR=examples/dynamic_only

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
examples: build-basic build-server build-resilient build-dynamic-resilient build-dynamic-only

# Build basic example
build-basic:
	@echo "Building basic example..."
	@mkdir -p $(BUILD_DIR)/examples
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/examples/basic ./$(EXAMPLES_DIR)/basic

# Build server
build-server:
	@echo "Building server..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/server ./$(SERVER_DIR)

# Build resilient example
build-resilient:
	@echo "Building resilient example..."
	@mkdir -p $(BUILD_DIR)/examples
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/examples/resilient ./$(RESILIENT_DIR)

# Build dynamic-resilient example
build-dynamic-resilient:
	@echo "Building dynamic-resilient example..."
	@mkdir -p $(BUILD_DIR)/examples
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/examples/dynamic-resilient ./$(DYNAMIC_RESILIENT_DIR)

# Build dynamic-only example
build-dynamic-only:
	@echo "Building dynamic-only example..."
	@mkdir -p $(BUILD_DIR)/examples
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/examples/dynamic-only ./$(DYNAMIC_ONLY_DIR)

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

# Run resilient example
run-resilient:
	@echo "Running resilient example..."
	$(GO) run $(RESILIENT_DIR)/main.go

# Run dynamic-resilient example
run-dynamic-resilient:
	@echo "Running dynamic-resilient example..."
	$(GO) run $(DYNAMIC_RESILIENT_DIR)/main.go

# Run dynamic-only example
run-dynamic-only:
	@echo "Running dynamic-only example..."
	$(GO) run $(DYNAMIC_ONLY_DIR)/main.go

# Build gRPC server
build-grpc-server:
	@echo "Building gRPC server..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/grpc-server main.go

# Run gRPC server
run-grpc-server:
	@echo "Running gRPC server..."
	$(GO) run main.go server

# Default target
all: clean build test lint examples build-grpc-server

# Help
help:
	@echo "Available targets:"
	@echo "  build                - Build the main binary"
	@echo "  test                 - Run tests"
	@echo "  test-cover           - Run tests with coverage"
	@echo "  clean                - Clean build artifacts"
	@echo "  lint                 - Run linter"
	@echo "  fmt                  - Format code"
	@echo "  verify               - Verify dependencies"
	@echo "  tidy                 - Tidy dependencies"
	@echo "  examples             - Build all examples"
	@echo "  build-basic          - Build basic example"
	@echo "  build-server         - Build server"
	@echo "  build-resilient      - Build resilient example"
	@echo "  build-dynamic-resilient - Build dynamic-resilient example"
	@echo "  build-dynamic-only   - Build dynamic-only example"
	@echo "  build-grpc-server    - Build gRPC server"
	@echo "  run-basic            - Run basic example"
	@echo "  run-dynamic          - Run dynamic example"
	@echo "  run-server           - Run server"
	@echo "  run-resilient        - Run resilient example"
	@echo "  run-dynamic-resilient - Run dynamic-resilient example"
	@echo "  run-dynamic-only     - Run dynamic-only example"
	@echo "  run-grpc-server      - Run gRPC server"
	@echo "  all                  - Clean, build, test, lint, and build examples"
	@echo "  help                 - Show this help message"
