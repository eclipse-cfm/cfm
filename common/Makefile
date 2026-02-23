.PHONY: test test-coverage clean lint generate-mocks install-mockery

MOCKERY_CMD=$(shell which mockery || echo $(GOPATH)/bin/mockery)

generate-mocks:
	$(MOCKERY_CMD)

# This target only needs to be run if the query/Query.g4 grammar changes.
# Requires ANTLR (antlr.org) to be installed locally.
generate-query-grammar:
	cd query && antlr -Dlanguage=Go -package query -visitor -listener Query.g4

# Build settings
COVER_PROFILE=coverage.out

# Run all tests
test:
	gotestsum --format github-actions ./... -count 1 ./...

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