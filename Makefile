.PHONY: build clean install test run help

# Variables
BINARY_NAME=changelog-update
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR=build
GOFILES=$(shell find . -name "*.go" -type f)

# Build settings
LDFLAGS=-ldflags "-X main.version=${VERSION}"

# Default target
all: build

# Help target
help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  clean       - Remove build artifacts"
	@echo "  install     - Install to /usr/local/bin"
	@echo "  test        - Run tests"
	@echo "  run         - Run the application with arguments"
	@echo "  help        - Show this help message"

# Build the binary
build: $(BUILD_DIR)/$(BINARY_NAME)

$(BUILD_DIR)/$(BINARY_NAME): $(GOFILES)
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "✅ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@echo "🧹 Cleaned build artifacts"

# Install to system
install: build
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "✅ Installed to /usr/local/bin/$(BINARY_NAME)"

# Run tests
test:
	go test -v ./...

# Run the application (use like: make run ARGS="--tag v1.0.3")
run: build
	$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)