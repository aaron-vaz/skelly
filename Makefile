.PHONY: clean build release test lint deps setup_workspace $(RELEASE_OS)

# Build directories
BUILD_DIR := ${PWD}/build
BINARY_DIR := ${BUILD_DIR}/bin

# Binary name
BINARY_NAME := skelly

# Version information
VERSION := $(shell git describe --tags --always --dirty)

# Build flags
GCFLAGS := -gcflags "all=-trimpath=${PWD}"
ASMFLAGS := -asmflags "all=-trimpath=${PWD}"
LDFLAGS := -ldflags "-X github.com/aaron-vaz/skelly/internal/commands.version=${VERSION} -s -w"

# Release targets
RELEASE_OS := linux darwin
RELEASE_ARCH := amd64 arm64

# Default target
all: lint test build

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf ${BUILD_DIR}
	@go clean

# Setup workspace
${BUILD_DIR}:
	@echo "Setting up workspace..."
	@mkdir -p ${BINARY_DIR}

# Run tests with coverage
test: ${BUILD_DIR}
	@echo "Running tests..."
	@go test -v -race -coverprofile=${BUILD_DIR}/coverage.out ./...
	@go tool cover -html=${BUILD_DIR}/coverage.out -o ${BUILD_DIR}/coverage.html

# Build for current platform
build: ${BUILD_DIR}
	@echo "Building binary..."
	@go build ${GCFLAGS} ${ASMFLAGS} ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME} ./cmd/${BINARY_NAME}

# Build for all platforms
release: ${BUILD_DIR} test $(RELEASE_OS)
	@echo "Building release artifacts..."

$(RELEASE_OS):
	@for arch in $(RELEASE_ARCH); do \
		echo "Building for $@/$$arch..."; \
		GOOS=$@ GOARCH=$$arch go build ${GCFLAGS} ${ASMFLAGS} ${LDFLAGS} \
			-o ${BINARY_DIR}/${BINARY_NAME}_$@_$$arch ./cmd/${BINARY_NAME}; \
	done

# Run linting
lint: ${BUILD_DIR}
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Please run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	
