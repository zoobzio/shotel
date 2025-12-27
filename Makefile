.PHONY: test test-unit test-integration bench lint coverage clean all help install-tools install-hooks ci

# Default target
all: test lint

# Display help
help:
	@echo "aperture Development Commands"
	@echo "=========================="
	@echo ""
	@echo "Testing & Quality:"
	@echo "  make test             - Run all tests with race detector"
	@echo "  make test-unit        - Run unit tests only (root + testing/)"
	@echo "  make test-integration - Run integration tests"
	@echo "  make bench            - Run benchmarks"
	@echo "  make lint             - Run linters"
	@echo "  make lint-fix         - Run linters with auto-fix"
	@echo "  make coverage         - Generate coverage report (HTML)"
	@echo "  make check            - Run tests and lint (quick check)"
	@echo "  make ci               - Full CI simulation (tests + quality checks)"
	@echo ""
	@echo "Setup:"
	@echo "  make install-tools - Install required development tools"
	@echo "  make install-hooks - Install git pre-commit hook"
	@echo ""
	@echo "Other:"
	@echo "  make clean         - Clean generated files"
	@echo "  make all           - Run tests and lint (default)"

# Run all tests with race detector
test:
	@echo "Running all tests..."
	@go test -v -race ./...

# Run unit tests only (root package + testing helpers)
test-unit:
	@echo "Running unit tests..."
	@go test -v -race . ./testing

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@go test -v -race ./testing/integration/...

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem -benchtime=100ms -timeout=5m ./testing/benchmarks/...

# Run linters
lint:
	@echo "Running linters..."
	@golangci-lint run --config=.golangci.yml --timeout=5m

# Run linters with auto-fix
lint-fix:
	@echo "Running linters with auto-fix..."
	@golangci-lint run --config=.golangci.yml --fix

# Generate coverage report
coverage:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1
	@echo "Coverage report generated: coverage.html"

# Clean generated files
clean:
	@echo "Cleaning..."
	@rm -f coverage.out coverage.html
	@find . -name "*.test" -delete
	@find . -name "*.prof" -delete
	@find . -name "*.out" -delete

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.2

# Install git pre-commit hook
install-hooks:
	@echo "Installing git hooks..."
	@mkdir -p .git/hooks
	@echo '#!/bin/sh' > .git/hooks/pre-commit
	@echo 'make check' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hook installed"

# Quick check - run tests and lint
check: test lint
	@echo "All checks passed!"

# CI simulation - what CI runs locally
ci: clean lint test coverage
	@echo "Full CI simulation complete!"
