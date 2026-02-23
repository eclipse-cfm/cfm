.PHONY: test test-coverage clean lint

# Build settings
COVER_PROFILE=coverage.out
TEST_AGENT_BINARY=testagent
BUILD_DIR=bin
TEST_AGENT_PATH=./testagent/main.go

build-test-agent:
	go build -o $(BUILD_DIR)/$(TEST_AGENT_BINARY) $(TEST_AGENT_PATH)

# Run all tests
test:
	gotestsum --format pkgname-and-test-fails

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