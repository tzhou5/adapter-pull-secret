.DEFAULT_GOAL := help

# CGO_ENABLED=0 for static binary (MVP simplified build)
# Set to 1 for FIPS compliance in production if needed
CGO_ENABLED := 0
GOPATH ?= $(shell go env GOPATH)

# Go version
GO_VERSION := go1.23.9

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Container tool (podman or docker)
CONTAINER_TOOL ?= $(shell command -v podman 2>/dev/null || echo docker)

# Image configuration
IMAGE_REGISTRY ?= quay.io/openshift-hyperfleet
IMAGE_NAME ?= pull-secret
IMAGE_TAG ?= $(VERSION)

# Dev image configuration
QUAY_USER ?=
DEV_TAG ?= dev-$(COMMIT)

# Enable Go modules
export GOPROXY=https://proxy.golang.org
export GOPRIVATE=gitlab.cee.redhat.com

####################
# Help
####################

help: ## Display this help
	@echo ""
	@echo "HyperFleet MVP - Pull Secret Job"
	@echo ""
	@echo "Build Targets:"
	@echo "  make build                compile pull-secret binary"
	@echo "  make test                 run unit tests with coverage"
	@echo "  make test-integration     run integration tests (requires podman)"
	@echo "  make lint                 run golangci-lint"
	@echo "  make fmt                  format code with gofmt/goimports"
	@echo "  make mod-tidy             tidy Go module dependencies"
	@echo "  make verify               run all verification checks (lint + test)"
	@echo "  make image                build container image"
	@echo "  make image-push           build and push container image"
	@echo "  make image-dev            build and push to personal Quay registry"
	@echo "  make clean                delete temporary generated files"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make test"
	@echo "  make test-integration"
	@echo "  make lint"
	@echo "  make verify"
	@echo "  make image IMAGE_TAG=v1.0.0"
	@echo "  make image-push IMAGE_TAG=v1.0.0"
	@echo "  make image-dev QUAY_USER=myuser"
	@echo ""
.PHONY: help

####################
# Build Targets
####################

# Checks if a GOPATH is set, or emits an error message
check-gopath:
ifndef GOPATH
	$(error GOPATH is not set)
endif
.PHONY: check-gopath

# Build pull-secret binary (HYPERFLEET-444: renamed from 'binary' to 'build')
# CGO_ENABLED=0 produces a static binary (no libc dependency)
build: check-gopath
	@echo "Building pull-secret binary..."
	CGO_ENABLED=$(CGO_ENABLED) go build \
		-o pull-secret \
		./cmd/pull-secret
	@echo "Binary built: ./pull-secret"
	@go version | grep -q "$(GO_VERSION)" || \
		( \
			printf '\033[41m\033[97m\n'; \
			echo "* WARNING: Your go version is not the expected $(GO_VERSION) *" | sed 's/./*/g'; \
			echo "* WARNING: Your go version is not the expected $(GO_VERSION) *"; \
			echo "* WARNING: Your go version is not the expected $(GO_VERSION) *" | sed 's/./*/g'; \
			printf '\033[0m\n'; \
		)
.PHONY: build

####################
# Test & Lint Targets
####################

# Run unit tests with coverage
test:
	@echo "Running unit tests with coverage..."
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	@echo ""
	@echo "Coverage report generated: coverage.txt"
	@echo "View HTML coverage: go tool cover -html=coverage.txt"
	@echo ""
.PHONY: test

# Run integration tests (requires testcontainers/podman)
# Integration tests are located in ./test/integration/
test-integration:
	@echo "Running integration tests..."
	@if [ -d "./test/integration" ] && [ -n "$$(find ./test/integration -name '*_test.go' 2>/dev/null)" ]; then \
		go test -v -race -timeout=10m ./test/integration/...; \
	else \
		echo "No integration tests found in ./test/integration/"; \
		echo "Skipping integration tests (placeholder for future tests)"; \
	fi
	@echo ""
	@echo "Integration tests completed!"
	@echo ""
.PHONY: test-integration

# Run golangci-lint
# Install: https://golangci-lint.run/usage/install/
lint:
	@echo "Running golangci-lint..."
	@if which golangci-lint > /dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
	elif [ -x "$(GOPATH)/bin/golangci-lint" ]; then \
		$(GOPATH)/bin/golangci-lint run --timeout=5m; \
	else \
		echo "golangci-lint not found. Install: https://golangci-lint.run/usage/install/"; \
		echo "Or run: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin v1.61.0"; \
		exit 1; \
	fi
	@echo ""
	@echo "Linting passed!"
	@echo ""
.PHONY: lint

# Format code with gofmt and goimports
fmt:
	@echo "Formatting code..."
	@if which goimports > /dev/null 2>&1; then \
		goimports -w .; \
	else \
		gofmt -w .; \
	fi
	@echo "Formatting complete."
.PHONY: fmt

# Tidy Go module dependencies
mod-tidy:
	@echo "Tidying Go modules..."
	go mod tidy
	go mod verify
	@echo "Go modules tidied."
.PHONY: mod-tidy

# Run all verification checks (lint + test)
verify: lint test
	@echo "All verification checks passed!"
.PHONY: verify

####################
# Container Image Targets
####################

image: ## Build container image with configurable registry/tag
	@echo "Building image $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)..."
	$(CONTAINER_TOOL) build -t $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG) .
.PHONY: image

image-push: image ## Build and push container image
	@echo "Pushing image $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)..."
	$(CONTAINER_TOOL) push $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
.PHONY: image-push

image-dev: ## Build and push to personal Quay registry
	@if [ -z "$(QUAY_USER)" ]; then \
		echo "Error: QUAY_USER is not set"; \
		echo "Usage: make image-dev QUAY_USER=your-username"; \
		exit 1; \
	fi
	@echo "Building dev image quay.io/$(QUAY_USER)/$(IMAGE_NAME):$(DEV_TAG)..."
	$(CONTAINER_TOOL) build -t quay.io/$(QUAY_USER)/$(IMAGE_NAME):$(DEV_TAG) .
	@echo "Pushing dev image quay.io/$(QUAY_USER)/$(IMAGE_NAME):$(DEV_TAG)..."
	$(CONTAINER_TOOL) push quay.io/$(QUAY_USER)/$(IMAGE_NAME):$(DEV_TAG)
.PHONY: image-dev

####################
# Clean
####################

# Delete temporary files and build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f pull-secret
	rm -f *.exe *.dll *.so *.dylib
	rm -f coverage.txt coverage.html
	@echo "Clean complete."
.PHONY: clean
