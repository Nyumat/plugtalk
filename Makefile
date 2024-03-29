# Simple Makefile for a Go project

# Build the application
all: build

watch-css:
	@echo "Watching CSS..."
	@npx tailwindcss -i cmd/web/css/globals.css -o cmd/web/css/output.css --watch

build:
	@echo "Building..."
	@templ generate
	@go build -o main cmd/api/main.go

# Run the application
run:
	@go run cmd/api/main.go

# Test the application
test:
	@echo "Testing..."
	@go test ./tests -v

# Clean the binary
clean:
	@echo "Cleaning..."
	@rm -f main

# Live Reload
watch:
	@if command -v air > /dev/null; then \
	    air & \
		make watch-css; \
	    echo "Watching...";\
	else \
	    read -p "Go's 'air' is not installed on your machine. Do you want to install it? [Y/n] " choice; \
	    if [ "$$choice" != "n" ] && [ "$$choice" != "N" ]; then \
	        go install github.com/cosmtrek/air@latest; \
	        air; \
	        echo "Watching...";\
	    else \
	        echo "You chose not to install air. Exiting..."; \
	        exit 1; \
	    fi; \
	fi

# Phony targets
.PHONY: all build run test clean

# Information about the Makefile
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all        Build the application"
	@echo "  build      Build the application"
	@echo "  run        Run the application"
	@echo "  test       Test the application"
	@echo "  clean      Clean the binary"
	@echo "  watch      Live Reload"
	@echo "  help       Display this help message"
	@echo ""
	@echo "Example:"
	@echo "  make run"
	@echo "  make test"
	@echo "  make clean"
	@echo "  make watch"
	@echo "  make help"