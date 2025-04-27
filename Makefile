.PHONY: build run clean

# Variables
BINARY_NAME=ai-assistant
STATIC_DIR=static

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) *.go

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Clean the binary
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy

# Setup the project structure
setup:
	@echo "Setting up project structure..."
	mkdir -p $(STATIC_DIR)
	cp index.html $(STATIC_DIR)/
	cp style.css $(STATIC_DIR)/
	cp app.js $(STATIC_DIR)/
	cp audio-processor.js $(STATIC_DIR)/

# Development mode (watches for changes and rebuilds)
dev:
	@echo "Starting development mode..."
	go run *.go