.PHONY: all build run test test-cover fmt lint vet clean help

# Build the executable
build:
	go build -o dino
	chmod +x dino

# Run the game directly
run:
	go run main.go

# Build with optimizations
build-release:
	go build -ldflags="-s -w" -o dino
	chmod +x dino

test:
	go test ./...

test-cover:
	go test ./... -cover

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	staticcheck ./...

gosec:
	gosec -exclude=G404 ./...

check: fmt vet lint gosec

clean:
	rm -f dino

# Show help
help:
	@echo "Available commands:"
	@echo "  make build         - Build the executable"
	@echo "  make run           - Run the game directly"
	@echo "  make build-release - Build with optimizations"
	@echo "  make test          - Run tests"
	@echo "  make test-cover    - Run tests with coverage"
	@echo "  make fmt           - Format code"
	@echo "  make vet           - Lint with go vet"
	@echo "  make lint          - Lint with staticcheck"
	@echo "  make gosec         - Run security check"
	@echo "  make check         - Run all checks (fmt, vet, lint, gosec)"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make help          - Show this help message"
