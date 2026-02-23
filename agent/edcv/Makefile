cd.PHONY: build test clean run server

# Binary name
SERVER_BINARY=edcvagent

# Build settings
BUILD_DIR=bin
SERVER_PATH=./cmd/server/main.go

DOCKER_TAG=latest

# Environment variables
export CGO_ENABLED=0

# Install development tools
install-tools:

# Build the application
build: build-server

# Build the server
build-server:
	go build -o $(BUILD_DIR)/$(SERVER_BINARY) $(SERVER_PATH)

# Run tests
test:
	gotestsum --format github-actions ./... -count 1

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	go clean

# Run the server in development mode
dev-server: build-server
	./$(BUILD_DIR)/$(SERVER_BINARY)

# Run the server in production mode
server: build-server
	./$(BUILD_DIR)/$(SERVER_BINARY)

# Build for multiple platforms
build-all:
	# Server binaries
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(SERVER_BINARY)-linux-amd64 $(SERVER_PATH)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(SERVER_BINARY)-darwin-amd64 $(SERVER_PATH)
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(SERVER_BINARY)-darwin-arm64 $(SERVER_PATH)
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(SERVER_BINARY)-windows-amd64.exe $(SERVER_PATH)
