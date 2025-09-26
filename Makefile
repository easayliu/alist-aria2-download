.PHONY: build run clean test fmt vet deps

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