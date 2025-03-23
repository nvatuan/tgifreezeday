.PHONY: build run clean test docker docker-run check-today list-upcoming

# Go parameters
BINARY_NAME=tgifreezeday
GO=go
MAIN_PATH=./cmd/tgifreezeday

build:
	$(GO) build -o $(BINARY_NAME) $(MAIN_PATH)

run: build
	./$(BINARY_NAME)

test:
	$(GO) test -v ./...

clean:
	$(GO) clean
	rm -f $(BINARY_NAME)

# Docker commands
docker:
	docker build -t $(BINARY_NAME):latest .

docker-run:
	docker-compose up -d

docker-stop:
	docker-compose down

# Utility commands
check-today: build
	./$(BINARY_NAME) --check-today

list-upcoming: build
	./$(BINARY_NAME) --list-upcoming

list-upcoming-days: build
	./$(BINARY_NAME) --list-upcoming --days=$(DAYS)

config-example:
	cp config.yaml.example config.yaml

# Setup commands
setup-deps:
	$(GO) mod download

help:
	@echo "Make targets:"
	@echo "  build          - Build the binary"
	@echo "  run            - Build and run the application"
	@echo "  test           - Run tests"
	@echo "  clean          - Clean build artifacts"
	@echo "  docker         - Build Docker image"
	@echo "  docker-run     - Run with Docker Compose"
	@echo "  docker-stop    - Stop Docker Compose services"
	@echo "  check-today    - Check if today is a freeze day"
	@echo "  list-upcoming  - List upcoming freeze days (7 days by default)"
	@echo "  list-upcoming-days DAYS=14 - List upcoming freeze days with custom days"
	@echo "  config-example - Create config.yaml from example"
	@echo "  setup-deps     - Download dependencies"

# Default target
.DEFAULT_GOAL := help 