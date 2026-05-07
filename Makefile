BINARY_NAME=tgifreezeday
MAIN_PATH=./cmd/server
BIN_DIR=bin

.PHONY: all
all: build

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PATH)

.PHONY: serve
serve: build
	LOG_LEVEL=debug LOG_FORMAT=colored ./$(BIN_DIR)/$(BINARY_NAME)

.PHONY: test
test:
	go test ./... -v

.PHONY: coverage
coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic -v
	go tool cover -func=coverage.out

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html

.PHONY: deps
deps:
	go mod download
	go mod tidy

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build    - Build the server binary"
	@echo "  serve    - Build and run the server (debug mode)"
	@echo "  test     - Run tests"
	@echo "  coverage - Run tests with coverage"
	@echo "  clean    - Remove build artifacts"
	@echo "  deps     - Install and tidy dependencies"
