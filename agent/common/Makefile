cd.PHONY: test clean

# Build settings
BUILD_DIR=bin

# Environment variables
export CGO_ENABLED=0


# Run tests
test:
	$(TEST_CMD)

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	go clean
