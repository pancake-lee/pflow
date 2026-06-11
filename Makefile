.PHONY: build env clean vet test dev run

# Default target
.DEFAULT_GOAL := build

# Directories
BIN_DIR := bin
WEB_DIR := web

# Binary name
BINARY := $(BIN_DIR)/pflow

## build: compile frontend and backend, output to bin/
build:
	@echo "==> Building frontend (Vue SPA)..."
	cd $(WEB_DIR) && npm run build
	@echo "==> Building backend (Go)..."
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) ./cmd/pflow/
	@echo "==> Done: $(BINARY)"

## env: install all dependencies (Go + Node.js)
env:
	@echo "==> Installing Go dependencies..."
	go mod download
	@echo "==> Installing Node.js dependencies..."
	cd $(WEB_DIR) && npm install --no-audit --no-fund
	@echo "==> Done."

## vet: run Go static analysis
vet:
	go vet ./...

## test: run Go tests
test:
	go test ./...

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR)
	cd $(WEB_DIR) && rm -rf dist

## run: build and start the server
run: build
	$(BINARY) serve

## dev: start Go API server (for use with 'cd web && npm run dev')
dev:
	go run ./cmd/pflow/ serve
