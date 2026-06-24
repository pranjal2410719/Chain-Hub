# ─────────────────────────────────────────────────────────────────────────────
# ChainHub — Multi-AI CLI Orchestrator
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: build run clean test install fmt lint help mod dev docker

APP_NAME   := chainhub
BUILD_DIR  := ./build
MAIN_PATH  := ./cmd/chainhub
VERSION    := 0.1.0
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GO         := go
LDFLAGS    := -ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# ─── Targets ─────────────────────────────────────────────────────────────────

help: ## Show this help message
	@echo ""
	@echo "  🔗 ChainHub — Build & Development Commands"
	@echo "  ───────────────────────────────────────────"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""

build: ## Build the chainhub binary
	@echo "🔨 Building $(APP_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "✅ Built: $(BUILD_DIR)/$(APP_NAME)"

run: build ## Build and run chainhub
	$(BUILD_DIR)/$(APP_NAME)

install: ## Install chainhub globally via go install
	@echo "📦 Installing $(APP_NAME)..."
	$(GO) install $(LDFLAGS) $(MAIN_PATH)
	@echo "✅ Installed to $(shell go env GOPATH)/bin/$(APP_NAME)"

clean: ## Remove build artifacts
	@echo "🧹 Cleaning..."
	rm -rf $(BUILD_DIR)
	@echo "✅ Clean"

test: ## Run all tests with verbose output
	@echo "🧪 Running tests..."
	$(GO) test ./... -v -race -count=1

test-cover: ## Run tests with coverage report
	@echo "📊 Running tests with coverage..."
	$(GO) test ./... -coverprofile=$(BUILD_DIR)/coverage.out
	$(GO) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "✅ Coverage report: $(BUILD_DIR)/coverage.html"

fmt: ## Format all Go source files
	@echo "✨ Formatting..."
	$(GO) fmt ./...
	@echo "✅ Formatted"

lint: ## Run golangci-lint
	@echo "🔍 Linting..."
	golangci-lint run ./...

vet: ## Run go vet
	@echo "🔍 Vetting..."
	$(GO) vet ./...

mod: ## Tidy Go module dependencies
	@echo "📦 Tidying modules..."
	$(GO) mod tidy
	@echo "✅ Modules tidied"

dev: ## Run in development mode with a test pipeline
	@echo "🚀 Starting development run..."
	$(GO) run $(MAIN_PATH) run "test pipeline"

init-workspace: ## Initialize a sample ChainHub workspace
	@echo "🏗️  Initializing workspace..."
	$(GO) run $(MAIN_PATH) init
	@echo "✅ Workspace ready"

version: ## Print version information
	@echo "$(APP_NAME) v$(VERSION) ($(GIT_COMMIT)) built $(BUILD_TIME)"
