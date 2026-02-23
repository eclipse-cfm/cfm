.PHONY: test test-coverage clean lint

# Build settings
COVER_PROFILE=coverage.out

# Environment variables
export CGO_ENABLED=0

# Run all tests
test:
	$(TEST_CMD)

# Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=$(COVER_PROFILE) -covermode=atomic ./...
	go tool cover -html=$(COVER_PROFILE)

# Run linting
lint:
	golangci-lint run ./...

# Clean test artifacts
clean:
	rm -f $(COVER_PROFILE)
	go clean -testcache

# Run tests and linting
check: lint test