.PHONY: build env clean vet test go dev run install start stop status

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

# --------------------------------------------------
# 以下都只处理go的
# --------------------------------------------------

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

go:
	@echo "==> Building backend (Go)..."
	mkdir -p $(BIN_DIR)
	go build -o $(BINARY) ./cmd/pflow/
	@echo "==> Done: $(BINARY)"


## install: build and install pflow to $GOPATH/bin (or $GOBIN)
install: build
	go install ./cmd/pflow/
	@echo "==> pflow installed to $$(go env GOPATH)/bin/pflow"

# ── 服务管理 ──────────────────────────────────────────────────────

PID_DIR := .local/.pids
LOG_DIR := logs

## start: build and start pflow serve in background
start: build
	@# Graceful stop of any existing instance
	@make -s stop
	@mkdir -p $(PID_DIR) $(LOG_DIR)
	@echo "🚀 启动 pflow serve..."
	@nohup $(BINARY) serve > $(LOG_DIR)/pflow.log 2>&1 & echo $$! > $(PID_DIR)/pflow.pid
	@echo "  ✓ pflow (pid $$(cat $(PID_DIR)/pflow.pid))"
	@echo "📋 日志: $(LOG_DIR)/pflow.log"
	@echo "   停止: make stop"

## stop: stop pflow serve
stop:
	@pid_file="$(PID_DIR)/pflow.pid"; \
	if [ -f "$$pid_file" ]; then \
		pid=$$(cat "$$pid_file"); \
		if kill -0 $$pid 2>/dev/null; then \
			echo "🛑 停止 pflow (pid $$pid)..."; \
			kill $$pid 2>/dev/null || true; \
			sleep 0.3; \
			kill -9 $$pid 2>/dev/null || true; \
			echo "✅ pflow 已停止"; \
		else \
			echo "  - pflow (pid $$pid 已退出)"; \
		fi; \
		rm -f "$$pid_file"; \
	else \
		echo "  - pflow (未找到 pid 文件)"; \
	fi

## status: show pflow serve status
status:
	@pid_file="$(PID_DIR)/pflow.pid"; \
	if [ -f "$$pid_file" ]; then \
		pid=$$(cat "$$pid_file"); \
		if kill -0 $$pid 2>/dev/null; then \
			echo "📊 pflow ● 运行中 (pid $$pid)"; \
		else \
			echo "📊 pflow ○ pid 文件存在但进程已退出"; \
		fi; \
	else \
		echo "📊 pflow ○ 未启动"; \
	fi
