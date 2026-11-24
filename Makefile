.PHONY: help build test lint clean docker-build docker-clean install-tools generate-mocks install-mockery

#==============================================================================
# Multi-Service Orchestration - Delegates to Individual Makefiles
#==============================================================================

# Service directories
ASSEMBLY_DIR=assembly
COMMON_DIR=common
PMANAGER_DIR=pmanager
KIND_CLUSTER_NAME=edcv

E2E_DIR=e2e

# Docker settings
DOCKER_REGISTRY=ghcr.io/eclipse-cfm/cfm/
DOCKER_TAG=latest

# TEST OUTPUT CONFIG
TEST_FORMAT=dots-v2
export TEST_CMD=gotestsum --format $(TEST_FORMAT) -- -count=1 ./...

#==============================================================================
# Help
#==============================================================================

help:
	@echo "CFM Make Commands:"
	@echo ""
	@echo "Build Commands:"
	@echo "  build          - Build all services"
	@echo "  build-pmanager - Build pmanager service"
	@echo "  build-all      - Build all services for all platforms"
	@echo ""
	@echo "Test Commands:"
	@echo "  test           - Run tests for all services"
	@echo "  test-pmanager  - Test pmanager service"
	@echo "  lint           - Run golangci-lint on all modules"
	@echo ""
	@echo "Development Commands:"
	@echo "  dev-pmanager   - Run pmanager in development mode"
	@echo "  clean          - Clean all build artifacts"
	@echo ""
	@echo "Docker Commands:"
	@echo "  docker-build   - Build all Docker images"
	@echo "  docker-clean   - Remove all Docker images"
	@echo ""
	@echo "Tool Commands:"
	@echo "  install-tools  - Install development tools for all services"
	@echo "  generate-mocks - Generate mocks for all services"

#==============================================================================
# Build Commands - Delegate to Service Makefiles
#==============================================================================

build:
	@echo "Building all modules..."
	$(MAKE) -C $(PMANAGER_DIR) build

build-pmanager:
	@echo "Building pmanager..."
	$(MAKE) -C $(PMANAGER_DIR) build

build-all:
	@echo "Building all modules for all platforms..."
	$(MAKE) -C $(PMANAGER_DIR) build-all

#==============================================================================
# Test Commands - Delegate to Service Makefiles
#==============================================================================

test: install-gotestsum
	@echo "Testing all services..."
	$(MAKE) -C $(COMMON_DIR) test
	$(MAKE) -C $(PMANAGER_DIR) test
	$(MAKE) -C $(E2E_DIR) test
	$(MAKE) -C $(ASSEMBLY_DIR) test

lint: install-golangci-lint
	@echo "Linting all modules..."
	golangci-lint run ./...

test-common:
	@echo "Testing common..."
	$(MAKE) -C $(COMMON_DIR) test

test-pmanager:
	@echo "Testing pmanager..."
	$(MAKE) -C $(PMANAGER_DIR) test

#==============================================================================
# Development Commands - Delegate to Service Makefiles
#==============================================================================

dev-pmanager:
	@echo "Starting pmanager in development mode..."
	$(MAKE) -C $(PMANAGER_DIR) dev-server

clean:
	@echo "Cleaning all services..."
	$(MAKE) -C $(COMMON_DIR) clean
	$(MAKE) -C $(PMANAGER_DIR) clean

#==============================================================================
# Tool Commands - Delegate to Service Makefiles
#==============================================================================

install-mockery:
	go install github.com/vektra/mockery/v2@v2.53.7

install-golangci-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

install-gotestsum:
	go install gotest.tools/gotestsum@latest

install-tools: install-mockery install-gotestsum
	@echo "Installing tools for all services..."
	$(MAKE) -C $(PMANAGER_DIR) install-tools

generate-mocks: install-mockery
	@echo "Generating mocks for all services..."
	$(MAKE) -C $(COMMON_DIR) generate-mocks
	$(MAKE) -C $(PMANAGER_DIR) generate-mocks

generate-docs:
	$(MAKE) -C $(PMANAGER_DIR) generate-docs

#==============================================================================
# Docker Commands - Handled at Top Level
#==============================================================================

docker-build: docker-build-pmanager

docker-build-pmanager:
	@echo "Building pmanager Docker image..."
	docker buildx build -f docker/Dockerfile.pmanager.dockerfile -t $(DOCKER_REGISTRY)pmanager:$(DOCKER_TAG) .

docker-clean: docker-clean-pmanager

docker-clean-pmanager:
	docker rmi $(DOCKER_REGISTRY)pmanager:$(DOCKER_TAG) || true

#==============================================================================
# Load images into KinD Cluster
#==============================================================================

load-into-kind-pmanager: docker-build-pmanager
	kind load docker-image -n $(KIND_CLUSTER_NAME) $(DOCKER_REGISTRY)pmanager:$(DOCKER_TAG)

# builds and loads all images into KinD cluster. Will require kind to be installed and a kind cluster named KIND_CLUSTER_NAME running.
load-into-kind: docker-build
	kind load docker-image -n $(KIND_CLUSTER_NAME) $$(docker images --format "{{.Repository}}:{{.Tag}}" | grep '^$(DOCKER_REGISTRY).*:$(DOCKER_TAG)')

#==============================================================================
# Combined Commands
#==============================================================================

all: build docker-build
	@echo "Built all services and Docker images"

deploy: build-all docker-build
	@echo "Built all services for all platforms and Docker images"

dev-setup: install-tools generate-mocks build
	@echo "Development environment ready for all services"
