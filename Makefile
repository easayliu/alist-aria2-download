.PHONY: build run clean test fmt vet deps build-linux build-linux-amd64 build-linux-arm64 package-linux

# Build variables
BINARY_NAME=alist-aria2-download
BUILD_DIR=build

# Build the application
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

# Run the application
run:
	go run ./cmd/server

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	go clean

# Run tests
test:
	go test -v ./...

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Development setup
dev-setup: deps
	@echo "Development environment setup complete"

# Run with hot reload (requires air)
dev:
	air

# Build for production
build-prod:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

# Docker build
docker-build:
	docker build -t $(BINARY_NAME) .

# Install development tools
install-tools:
	go install github.com/cosmtrek/air@latest
	go install github.com/swaggo/swag/cmd/swag@latest

# Generate Swagger documentation
swagger:
	swag init -g cmd/server/main.go -o docs

# Build for Linux (amd64)
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-w -s' -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/server

# Build for Linux amd64
build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-w -s' -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/server

# Build for Linux arm64
build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -ldflags '-w -s' -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/server

# Build for all Linux architectures
build-linux-all: build-linux-amd64 build-linux-arm64
	@echo "Linux builds completed for amd64 and arm64"

# Package Linux builds into tar.gz
package-linux: build-linux-all
	@mkdir -p $(BUILD_DIR)/packages
	tar -czf $(BUILD_DIR)/packages/$(BINARY_NAME)-linux-amd64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-linux-amd64
	tar -czf $(BUILD_DIR)/packages/$(BINARY_NAME)-linux-arm64.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-linux-arm64
	@echo "Linux packages created in $(BUILD_DIR)/packages/"