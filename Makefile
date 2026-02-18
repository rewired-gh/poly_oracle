.PHONY: build test run clean docker-build install

# Binary name
BINARY_NAME=polyoracle
BINARY_PATH=bin/$(BINARY_NAME)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build the binary
build:
	$(GOBUILD) -o $(BINARY_PATH) ./cmd/$(BINARY_NAME)

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -cover ./...

# Run the application
run: build
	./$(BINARY_PATH) --config configs/config.yaml

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf bin/
	rm -rf data/

# Install dependencies
install:
	$(GOMOD) download
	$(GOMOD) tidy

# Docker build
docker-build:
	docker build -t $(BINARY_NAME):latest .

# Docker run
docker-run:
	docker run -d --name $(BINARY_NAME) \
		-v $(PWD)/configs:/app/configs \
		-v $(PWD)/data:/app/data \
		$(BINARY_NAME):latest

# Lint check
lint:
	golangci-lint run

# Format code
fmt:
	gofmt -s -w .

# Development mode with auto-reload (requires entr or similar tool)
dev:
	ls -d internal/**/*.go cmd/**/*.go | entr -r $(GOBUILD) -o $(BINARY_PATH) ./cmd/$(BINARY_NAME) && ./$(BINARY_PATH)

# Generate example config
generate-config:
	@echo "Generating example configuration..."
	@./$(BINARY_PATH) --generate-config > configs/config.yaml.example

# All dependencies
all: install build test
