.PHONY: build test clean run server dev-server

# Binary name
BINARY_NAME=assembly
SERVER_BINARY=tmanager

# Build settings
BUILD_DIR=bin
MAIN_PATH=./cmd/main.go
SERVER_PATH=./cmd/server/main.go

OPENAPI_PATH=./cmd/openapi/main.go

# Environment variables
export CGO_ENABLED=0

# Build the application
build: build-server

# Build the server
build-server:
	go build -o $(BUILD_DIR)/$(SERVER_BINARY) $(SERVER_PATH)

# Install development tools
install-tools:

generate-docs:
	go run $(OPENAPI_PATH)

# Run tests
test:
	gotestsum --format pkgname-and-test-fails

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
	
