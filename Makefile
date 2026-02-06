# ============================================
# Config
# ============================================
.DEFAULT_GOAL := help

CURRENT_PATH := $(shell pwd)

# Automatically collect demos from directories
DEMOS := $(wildcard tutorial/*)
DEMO_INDEX ?= 3

PORT    ?= 8106
MOVIE   ?= false

ifeq ($(OS),Windows_NT)
SPX_BIN := ./dist/bin/spx.exe
else
SPX_BIN := ./dist/bin/spx
endif

# ============================================
# Help
# ============================================
help: ## Show available commands
	echo "Detected demos: $(DEMOS)"
	@echo "Make Commands:"
	@echo "================================"
	@grep -E '^[a-zA-Z0-9._-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  make %-25s %s\n", $$1, $$2}'
	@echo ""
	@echo "Demo targets via index:"
	@i=1; \
	for demo in $(DEMOS); do \
		echo "  make run DEMO_INDEX=$$i            # Run $$demo"; \
		echo "  make run-web DEMO_INDEX=$$i        # Run web $$demo"; \
		echo "  make run-web-worker DEMO_INDEX=$$i # Run web-worker $$demo"; \
		echo "  make run-editor DEMO_INDEX=$$i     # Run editor $$demo"; \
		i=$$((i+1)); \
	done

# ============================================
# Demo Commands
# ============================================
list-demos: ## List all demos with index
	@i=1; \
	for demo in $(DEMOS); do \
		echo "$$i: $$demo"; \
		i=$$((i+1)); \
	done

# ============================================
# Build Commands
# ============================================
.PHONY: build build-web build-native build-all clean

build: ## Build spx and spxrun
	./scripts/build.sh

build-web: ## Build spx and web runtime assets
	./scripts/build.sh --web

build-native: ## Build spx and native runtime libraries
	./scripts/build.sh --native

build-all: ## Build all components
	./scripts/build.sh --all

clean: ## Remove dist artifacts
	rm -rf dist/

# ============================================
# Setup Commands
# ============================================
.PHONY: setup setup-engines setup-web

setup: ## Build all and download engines
	$(MAKE) build-all
	$(MAKE) setup-engines
	@echo "Setup completed. Use 'make setup-web MODE=normal' to setup web templates."

setup-engines: ## Download engines to dist/share/engines
	./scripts/setup-engines.sh

setup-web: ## Generate web templates (MODE=normal|worker|minigame|miniprogram)
ifndef MODE
	$(error MODE is not set! Usage: make setup-web MODE=normal)
endif
	./scripts/setup-web-template.sh $(MODE)

# ============================================
# Run Commands (by index)
# ============================================
define GET_DEMO
$(word $(DEMO_INDEX),$(DEMOS))
endef

editor: ## Open demo in editor: make editor DEMO_INDEX=N
ifndef DEMO_INDEX
	$(error DEMO_INDEX is not set! Usage: make editor DEMO_INDEX=N)
endif
	@DEMO=$(GET_DEMO); \
	echo "Opening editor for demo #$(DEMO_INDEX): $$DEMO"; \
	cd $$DEMO && $(SPX_BIN) editor -movie=$(MOVIE)

run: ## Run demo on PC: make run DEMO_INDEX=N
ifndef DEMO_INDEX
	$(error DEMO_INDEX is not set! Usage: make run DEMO_INDEX=N)
endif
	@DEMO=$(GET_DEMO); \
	echo "Running demo #$(DEMO_INDEX): $$DEMO"; \
	cd $$DEMO && $(SPX_BIN) run -movie=$(MOVIE)

run-editor: ## Run demo in editor mode: make run-editor DEMO_INDEX=N
ifndef DEMO_INDEX
	$(error DEMO_INDEX is not set! Usage: make run-editor DEMO_INDEX=N)
endif
	@DEMO=$(GET_DEMO); \
	echo "Running editor demo #$(DEMO_INDEX): $$DEMO"; \
	cd $$DEMO && $(SPX_BIN) rune -movie=$(MOVIE)

run-web: ## Run demo on web: make run-web DEMO_INDEX=N
ifndef DEMO_INDEX
	$(error DEMO_INDEX is not set! Usage: make run-web DEMO_INDEX=N)
endif
	@DEMO=$(GET_DEMO); \
	echo "Running web demo #$(DEMO_INDEX): $$DEMO"; \
	$(MAKE) stop && $(MAKE) build-web && \
	cd $$DEMO && $(SPX_BIN) clear && $(SPX_BIN) runweb -serveraddr=":$(PORT)"

run-web-worker: ## Run demo on web worker: make run-web-worker DEMO_INDEX=N
ifndef DEMO_INDEX
	$(error DEMO_INDEX is not set! Usage: make run-web-worker DEMO_INDEX=N)
endif
	@DEMO=$(GET_DEMO); \
	echo "Running web worker mode: demo #$(DEMO_INDEX): $$DEMO"; \
	$(MAKE) stop && $(MAKE) build-web && \
	cd $$DEMO && $(SPX_BIN) clear && $(SPX_BIN) runwebworker -serveraddr=":$(PORT)"

# ============================================
# Utility Commands
# ============================================
format: ## Format Go code
	go fmt ./...

generate: ## Generate code
	cd ./pkg/gdspx/cmd/codegen && go run . && cd $(CURRENT_PATH) && \
	go generate ./cmd/spxrun/runner && \
	$(MAKE) format

export-pack: ## Export runtime pck file
	SPX_BIN=$(SPX_BIN) ./pkg/gdspx/tools/make_util.sh exportpack

export-web: ## Export web engine. Usage: make export-web MODE=normal
	$(MAKE) build-web
	@if [ -z "$(MODE)" ]; then \
		EXPORT_MODE=normal; \
	else \
		EXPORT_MODE=$(MODE); \
	fi; \
	SPX_BIN=$(SPX_BIN) ./pkg/gdspx/tools/make_util.sh exportweb $$EXPORT_MODE

stop: ## Stop running processes
	@echo "Stopping running processes..."
	@if [ "$$OS" = "Windows_NT" ]; then \
		taskkill /F /FI "IMAGENAME eq python.exe" 2>NUL || true; \
		taskkill /F /FI "IMAGENAME eq python3.exe" 2>NUL || true; \
	else \
		PIDS=$$(pgrep -f gdspx_web_server.py || true); \
		if [ -n "$$PIDS" ]; then kill -9 $$PIDS; fi \
	fi
	@echo "Processes stopped."
