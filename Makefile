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
	go test ./...

# Run the application
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BIN_DIR)/$(BINARY_NAME)

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
	@echo "  build    - Build the application to $(BIN_DIR)/"
	@echo "  clean    - Remove build artifacts"
	@echo "  test     - Run tests"
	@echo "  run      - Build and run the application"
	@echo "  deps     - Install and tidy dependencies"
	@echo "  help     - Show this help message" 