# Variables
BINARY_NAME=tgifreezeday
MAIN_PATH=./cmd/tgifreezeday
BIN_DIR=bin

# Default target
.PHONY: all
all: build

# Build the application
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Built $(BIN_DIR)/$(BINARY_NAME)"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	@echo "Cleaned $(BIN_DIR)"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test ./... -v

# Run the application
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BIN_DIR)/$(BINARY_NAME) $(ARGS)

# Sync freeze day blockers to calendar
.PHONY: sync
sync: build
	@echo "Syncing freeze day blockers..."
	LOG_LEVEL=debug LOG_FORMAT=colored ./$(BIN_DIR)/$(BINARY_NAME) sync

# Wipe all blockers in range
.PHONY: wipe-blockers
wipe-blockers: build
	@echo "Wiping all blockers..."
	LOG_LEVEL=debug LOG_FORMAT=colored ./$(BIN_DIR)/$(BINARY_NAME) wipe-blockers

# List all blockers in range
.PHONY: list-blockers
list-blockers: build
	@echo "Listing all blockers..."
	LOG_LEVEL=debug LOG_FORMAT=colored ./$(BIN_DIR)/$(BINARY_NAME) list-blockers

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application to $(BIN_DIR)/"
	@echo "  clean         - Remove build artifacts"
	@echo "  test          - Run tests"
	@echo "  run           - Build and run the application (use ARGS=\"subcommand\" to pass arguments)"
	@echo "  sync          - Build and run sync command"
	@echo "  wipe-blockers - Build and run wipe-blockers command"
	@echo "  list-blockers - Build and run list-blockers command"
	@echo "  deps          - Install and tidy dependencies"
	@echo "  help          - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  LOG_LEVEL     - Set log level (debug, info, warn, error, fatal, panic). Default: info"
	@echo "  LOG_FORMAT    - Set log format (json, text, colored). Default: json"
	@echo "  Examples:"
	@echo "    LOG_LEVEL=debug make sync"
	@echo "    LOG_FORMAT=colored LOG_LEVEL=debug make sync" 