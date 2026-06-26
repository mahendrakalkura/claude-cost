.PHONY: build run lint test clean

# Build the binary as `main` in the current directory.
build:
	go build -o main .

# Run with --no-fetch for speed (uses cached/fallback pricing).
run: build
	./main --no-fetch

# Run golangci-lint on all packages.
lint:
	golangci-lint run ./...

# Run the full test suite (unit, integration, and e2e).
test:
	go test ./...

# Remove the built binary.
clean:
	gio trash main 2>/dev/null || true
