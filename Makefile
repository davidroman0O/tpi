# Makefile for tpi 

.PHONY: all build install clean test lint test-hw test-hw-control

all: build

build:
	@echo "Building TPI CLI..."
	@go build -o bin/tpi ./cmd/tpi

install:
	@echo "Installing TPI CLI..."
	@go install ./cmd/tpi

clean:
	@echo "Cleaning..."
	@rm -rf bin

test:
	@echo "Running tests..."
	@go test -v ./...

test-hw:
	@echo "Running hardware tests..."
	@go test -v -run "TestHardware[^C]" # Run all hardware tests except control tests

test-hw-control:
	@echo "Running hardware control tests..."
	@echo "WARNING: This will affect hardware state!"
	@read -p "Are you sure you want to continue? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		TPI_TEST_POWER=1 TPI_TEST_USB=1 go test -v -run "TestHardwareControl"; \
	else \
		echo "Hardware control tests skipped."; \
	fi

lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

help:
	@echo "Available targets:"
	@echo "  all            - Build the TPI CLI (default)"
	@echo "  build          - Build the TPI CLI"
	@echo "  install        - Install the TPI CLI"
	@echo "  clean          - Remove build artifacts"
	@echo "  test           - Run tests"
	@echo "  test-hw        - Run hardware tests (read-only)"
	@echo "  test-hw-control- Run hardware control tests (will modify hardware state)"
	@echo "  lint           - Run the linter"
	@echo "  help           - Show this help" 