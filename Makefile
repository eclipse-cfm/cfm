.PHONY: help build test clean docker-build docker-clean install-tools generate-mocks install-mockery

#==============================================================================
# Multi-Service Orchestration - Delegates to Individual Makefiles
#==============================================================================

# Service directories
COMMON_DIR=common
PMANAGER_DIR=pmanager
TMANAGER_DIR=tmanager
EDCV_DIR=agent/edcv
KEYCLOAK_DIR=agent/keycloak
REG_DIR=agent/registration
ONBOARDING_DIR=agent/onboarding
KIND_CLUSTER_NAME=edcv

E2E_DIR=e2e

# Docker settings
DOCKER_REGISTRY=ghcr.io/metaform/connector-fabric-manager/
DOCKER_TAG=latest

#==============================================================================
# Help
#==============================================================================

help:
	@echo "CFM Make Commands:"
	@echo ""
	@echo "Build Commands:"
	@echo "  build          - Build all services"
	@echo "  build-pmanager - Build pmanager service"
	@echo "  build-tmanager - Build tmanager service"
	@echo "  build-all      - Build all services for all platforms"
	@echo ""
	@echo "Test Commands:"
	@echo "  test           - Run tests for all services"
	@echo "  test-pmanager  - Test pmanager service"
	@echo "  test-tmanager  - Test tmanager service"
	@echo ""
	@echo "Development Commands:"
	@echo "  dev-pmanager   - Run pmanager in development mode"
	@echo "  dev-tmanager   - Run tmanager in development mode"
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
	$(MAKE) -C $(TMANAGER_DIR) build
	$(MAKE) -C $(EDCV_DIR) build
	$(MAKE) -C $(KEYCLOAK_DIR) build
	$(MAKE) -C $(REG_DIR) build
	$(MAKE) -C $(ONBOARDING_DIR) build

build-pmanager:
	@echo "Building pmanager..."
	$(MAKE) -C $(PMANAGER_DIR) build

build-tmanager:
	@echo "Building tmanager..."
	$(MAKE) -C $(TMANAGER_DIR) build

build-all:
	@echo "Building all modules for all platforms..."
	$(MAKE) -C $(PMANAGER_DIR) build-all
	$(MAKE) -C $(TMANAGER_DIR) build-all
	$(MAKE) -C $(EDCV_DIR) build-all
	$(MAKE) -C $(KEYCLOAK_DIR) build-all
	$(MAKE) -C $(REG_DIR) build-all
	$(MAKE) -C $(ONBOARDING_DIR) build-all

#==============================================================================
# Test Commands - Delegate to Service Makefiles
#==============================================================================

test: install-gotestsum
	@echo "Testing all services..."
	$(MAKE) -C $(COMMON_DIR) test
	$(MAKE) -C $(PMANAGER_DIR) test
	$(MAKE) -C $(TMANAGER_DIR) test
	$(MAKE) -C $(EDCV_DIR) test
	$(MAKE) -C $(E2E_DIR) test
	$(MAKE) -C $(KEYCLOAK_DIR) test
	$(MAKE) -C $(REG_DIR) test
	$(MAKE) -C $(ONBOARDING_DIR) test

test-common:
	@echo "Testing common..."
	$(MAKE) -C $(COMMON_DIR) test

test-pmanager:
	@echo "Testing pmanager..."
	$(MAKE) -C $(PMANAGER_DIR) test

test-tmanager:
	@echo "Testing tmanager..."
	$(MAKE) -C $(TMANAGER_DIR) test

test-edcv:
	@echo "Testing EDC-V agent..."
	$(MAKE) -C $(EDCV_DIR) test

test-reg:
	@echo "Testing Registration agent..."
	$(MAKE) -C $(REG_DIR) test

test-onboarding:
	@echo "Testing Onboarding agent..."
	$(MAKE) -C $(ONBOARDING_DIR) test

#==============================================================================
# Development Commands - Delegate to Service Makefiles
#==============================================================================

dev-pmanager:
	@echo "Starting pmanager in development mode..."
	$(MAKE) -C $(PMANAGER_DIR) dev-server

dev-tmanager:
	@echo "Starting tmanager in development mode..."
	$(MAKE) -C $(TMANAGER_DIR) dev-server

clean:
	@echo "Cleaning all services..."
	$(MAKE) -C $(COMMON_DIR) clean
	$(MAKE) -C $(PMANAGER_DIR) clean
	$(MAKE) -C $(TMANAGER_DIR) clean
	$(MAKE) -C $(EDCV_DIR) clean
	$(MAKE) -C $(REG_DIR) clean
	$(MAKE) -C $(ONBOARDING_DIR) clean

#==============================================================================
# Tool Commands - Delegate to Service Makefiles
#==============================================================================

install-mockery:
	go install github.com/vektra/mockery/v2@latest

install-gotestsum:
	go install gotest.tools/gotestsum@latest

install-tools: install-mockery install-gotestsum
	@echo "Installing tools for all services..."
	$(MAKE) -C $(PMANAGER_DIR) install-tools
	$(MAKE) -C $(TMANAGER_DIR) install-tools

generate-mocks: install-mockery
	@echo "Generating mocks for all services..."
	$(MAKE) -C $(COMMON_DIR) generate-mocks
	$(MAKE) -C $(PMANAGER_DIR) generate-mocks


generate-docs:
	$(MAKE) -C $(TMANAGER_DIR) generate-docs
	$(MAKE) -C $(PMANAGER_DIR) generate-docs

#==============================================================================
# Docker Commands - Handled at Top Level
#==============================================================================

docker-build: docker-build-pmanager docker-build-tmanager docker-build-testagent docker-build-edcvagent docker-build-kcagent docker-build-regagent docker-build-obagent

docker-build-pmanager:
	@echo "Building pmanager Docker image..."
	docker buildx build -f docker/Dockerfile.pmanager.dockerfile -t $(DOCKER_REGISTRY)pmanager:$(DOCKER_TAG) .

docker-build-tmanager:
	@echo "Building tmanager Docker image..."
	docker buildx build -f docker/Dockerfile.tmanager.dockerfile -t $(DOCKER_REGISTRY)tmanager:$(DOCKER_TAG) .

docker-build-testagent:
	@echo "Building test agent Docker image..."
	docker buildx build -f docker/Dockerfile.testagent.dockerfile -t $(DOCKER_REGISTRY)testagent:$(DOCKER_TAG) .

docker-build-edcvagent:
	@echo "Building EDC-V agent Docker image..."
	docker buildx build -f docker/Dockerfile.edcvagent.dockerfile -t $(DOCKER_REGISTRY)edcvagent:$(DOCKER_TAG) .

docker-build-kcagent:
	@echo "Building Keycloak agent Docker image..."
	docker buildx build -f docker/Dockerfile.kcagent.dockerfile -t $(DOCKER_REGISTRY)kcagent:$(DOCKER_TAG) .

docker-build-regagent:
	@echo "Building Registration agent Docker image..."
	docker buildx build -f docker/Dockerfile.regagent.dockerfile -t $(DOCKER_REGISTRY)regagent:$(DOCKER_TAG) .

docker-build-obagent:
	@echo "Building Onboarding agent Docker image..."
	docker buildx build -f docker/Dockerfile.obagent.dockerfile -t $(DOCKER_REGISTRY)obagent:$(DOCKER_TAG) .

docker-clean: docker-clean-pmanager docker-clean-tmanager docker-clean-testagent docker-clean-regagent docker-clean-obagent

docker-clean-pmanager:
	docker rmi $(DOCKER_REGISTRY)pmanager:$(DOCKER_TAG) || true

docker-clean-tmanager:
	docker rmi $(DOCKER_REGISTRY)tmanager:$(DOCKER_TAG) || true

docker-clean-testagent:
	docker rmi $(DOCKER_REGISTRY)testagent:$(DOCKER_TAG) || true

docker-clean-edcvagent:
	docker rmi $(DOCKER_REGISTRY)edcvagent:$(DOCKER_TAG) || true

docker-clean-regagent:
	docker rmi $(DOCKER_REGISTRY)regagent:$(DOCKER_TAG) || true

docker-clean-obagent:
	docker rmi $(DOCKER_REGISTRY)obagent:$(DOCKER_TAG) || true

#==============================================================================
# Load images into KinD Cluster
#==============================================================================

load-into-kind-pmanager: docker-build-pmanager
	kind load docker-image -n $(KIND_CLUSTER_NAME) $(DOCKER_REGISTRY)pmanager:$(DOCKER_TAG)

load-into-kind-tmanager: docker-build-tmanager
	kind load docker-image -n $(KIND_CLUSTER_NAME) $(DOCKER_REGISTRY)tmanager:$(DOCKER_TAG)

load-into-kind-edcvagent: docker-build-edcvagent
	kind load docker-image -n $(KIND_CLUSTER_NAME) $(DOCKER_REGISTRY)edcvagent:$(DOCKER_TAG)

load-into-kind-kcagent: docker-build-kcagent
	kind load docker-image -n $(KIND_CLUSTER_NAME) $(DOCKER_REGISTRY)kcagent:$(DOCKER_TAG)

load-into-kind-obagent: docker-build-obagent
	kind load docker-image -n $(KIND_CLUSTER_NAME) $(DOCKER_REGISTRY)obagent:$(DOCKER_TAG)

load-into-kind-regagent: docker-build-regagent
	kind load docker-image -n $(KIND_CLUSTER_NAME) $(DOCKER_REGISTRY)regagent:$(DOCKER_TAG)

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
