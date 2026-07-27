#!/bin/bash
# SentinelCNAPP Development Script

.PHONY: dev build test lint clean proto proto-gen help

GO ?= go
BUF ?= buf
NODE ?= node
NPM ?= npm
DOCKER ?= docker

# ──────────────────────────────────────────────
# Development
# ──────────────────────────────────────────────

dev: ## Start development environment (Tilt + kind)
	@echo "Starting dev environment..."
	@tilt up

dev-down: ## Stop development environment
	@echo "Stopping dev environment..."
	@tilt down

# ──────────────────────────────────────────────
# Building
# ──────────────────────────────────────────────

build: ## Build all Go services
	@echo "Building all services..."
	$(GO) build ./...

build-service: ## Build a specific service: make build-service SVC=services/asset-inventory
	@echo "Building $(SVC)..."
	$(GO) build ./$(SVC)

# ──────────────────────────────────────────────
# Testing
# ──────────────────────────────────────────────

test: ## Run all tests
	@echo "Running tests..."
	$(GO) test ./... -v -count=1

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GO) test ./... -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -html=coverage.out -o coverage.html

test-race: ## Run tests with race detector
	$(GO) test ./... -race -count=1

# ──────────────────────────────────────────────
# Linting
# ──────────────────────────────────────────────

lint: ## Lint all code
	@echo "Linting Go code..."
	@golangci-lint run ./...
	@echo "Linting Protobuf..."
	@$(BUF) lint api/proto

lint-fix: ## Auto-fix linter issues
	@golangci-lint run ./... --fix

# ──────────────────────────────────────────────
# Protobuf
# ──────────────────────────────────────────────

proto-gen: ## Generate protobuf code
	@echo "Generating protobuf code..."
	@$(BUF) generate api/proto

proto-lint: ## Lint protobuf files
	@$(BUF) lint api/proto

proto-breaking: ## Check protobuf breaking changes
	@$(BUF) breaking api/proto --against .git

# ──────────────────────────────────────────────
# Frontend
# ──────────────────────────────────────────────

frontend-dev: ## Start frontend dev server
	$(NPM) --prefix frontend run dev

frontend-build: ## Build frontend for production
	$(NPM) --prefix frontend run build

frontend-lint: ## Lint frontend code
	$(NPM) --prefix frontend run lint

# ──────────────────────────────────────────────
# Docker
# ──────────────────────────────────────────────

docker-build: ## Build all Docker images
	@echo "Building Docker images..."
	@$(DOCKER) build -t sentinel-cnapp/asset-inventory:latest -f services/asset-inventory/Dockerfile .
	@$(DOCKER) build -t sentinel-cnapp/scanner-iac:latest -f services/scanner-iac/Dockerfile .

docker-build-service: ## Build a specific service Docker image: make docker-build-service SVC=asset-inventory
	@echo "Building Docker image for $(SVC)..."
	$(DOCKER) build -t sentinel-cnapp/$(SVC):latest -f services/$(SVC)/Dockerfile .

# ──────────────────────────────────────────────
# Cleanup
# ──────────────────────────────────────────────

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GO) clean ./...
	rm -rf coverage.out coverage.html
	rm -rf frontend/.next

# ──────────────────────────────────────────────
# Help
# ──────────────────────────────────────────────

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
