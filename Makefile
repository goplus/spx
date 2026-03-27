# ============================================
# Config
# ============================================
.DEFAULT_GOAL := help
.PHONY: help buildctl list-demos prepare-host prepare-web prepare-full build-dev install clean-assets download download-engine build-editor build-desktop build-web build-web-worker build-web-minigame build-web-miniprogram build-wasm build-wasm-opt build-android build-ios install-apk editor run run-editor run-web run-web-worker format generate export-pack export-web stop

export GODOT_SRC

BUILDCTL_BIN := ./.bin/buildctl$(shell go env GOEXE)
BUILDCTL_LAUNCHER := bash ./internal/cmd/buildctl/buildctl.sh
BUILDCTL_CMD := $(BUILDCTL_LAUNCHER)
BUILDCTL_TOOL_CMD := $(BUILDCTL_CMD) tool
BUILDCTL_ENGINE_DOWNLOAD_CMD := $(BUILDCTL_CMD) engine download
BUILDCTL_ENGINE_BUILD_CMD := $(BUILDCTL_CMD) engine build
BUILDCTL_RUNTIME_CMD := $(BUILDCTL_CMD) runtime
BUILDCTL_WORKFLOW_CMD := $(BUILDCTL_CMD) workflow

DEMO_INDEX ?= 3
APK_PROJECT_DIR ?= tutorial/00-Hello

PORT    ?= 8106
MOVIE   ?= false
WEB_MODE = $(or $(strip $(MODE)),normal)

# ============================================
# Help
# ============================================
buildctl: ## Build cached buildctl binary at ./.bin/buildctl
	@mkdir -p ./.bin
	go build -o $(BUILDCTL_BIN) ./internal/cmd/buildctl

help: ## Show available commands
	@echo "Make Commands:"
	@echo "================================"
	@grep -E '^[a-zA-Z0-9._-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  make %-25s %s\n", $$1, $$2}'
	@echo ""
	@echo "Demo targets via index:"
	@i=1; \
	for demo in tutorial/*; do \
		if [ -d "$$demo" ]; then \
			echo "  make run DEMO_INDEX=$$i            # Run $$demo"; \
			echo "  make run-web DEMO_INDEX=$$i        # Run web $$demo"; \
			echo "  make run-web-worker DEMO_INDEX=$$i # Run web-worker $$demo"; \
			echo "  make run-editor DEMO_INDEX=$$i     # Run editor $$demo"; \
			i=$$((i+1)); \
		fi; \
	done

# ============================================
# Prepare Commands
# ============================================
prepare-host: ## Prepare host assets, including editor/runtime files. Optional: GODOT_SRC=/abs/path/to/godot
	$(BUILDCTL_CMD) prepare --setup-mode runtime

prepare-full: ## Prepare host assets plus web export assets. Usage: make prepare-full [MODE=normal|worker|minigame|miniprogram] [GODOT_SRC=/abs/path/to/godot]
	$(BUILDCTL_CMD) prepare --setup-mode full --web-mode "$(WEB_MODE)"

prepare-web: ## Prepare web export assets for MODE, including the host editor required by exporttemplateweb. Usage: make prepare-web MODE=normal [GODOT_SRC=/abs/path/to/godot] (MODE: normal|worker|minigame|miniprogram)
	$(BUILDCTL_CMD) prepare --setup-mode web --web-mode "$(WEB_MODE)"

# ============================================
# Install & Download
# ============================================
install: ## Install SPX command
	$(BUILDCTL_TOOL_CMD) install

clean-assets: ## Remove installed SPX/Godot runtime assets from GOPATH/bin
	$(BUILDCTL_TOOL_CMD) clean-assets

download: ## Download engines. Optional: GODOT_SRC=/abs/path/to/godot
	$(BUILDCTL_ENGINE_DOWNLOAD_CMD) --runtime

download-engine: ## Download engine templates for specific platform. Usage: make download-engine PLATFORM=android|ios|web [MODE=normal|worker|minigame|miniprogram] [GODOT_SRC=/abs/path/to/godot]
	@echo "Downloading engine templates for platform: $(PLATFORM)"
	@set --; \
	if [ -n "$(PLATFORM)" ]; then \
		set -- "$$@" --platform "$(PLATFORM)"; \
	fi; \
	if [ -n "$(MODE)" ]; then \
		set -- "$$@" --mode "$(MODE)"; \
	fi; \
	$(BUILDCTL_ENGINE_DOWNLOAD_CMD) "$$@"


# ============================================
# Build Commands
# ============================================
build-dev: ## Build the full local development environment. Usage: make build-dev [MODE=normal|worker|minigame|miniprogram] [GODOT_SRC=/abs/path/to/godot]
	$(BUILDCTL_WORKFLOW_CMD) build-dev --web-mode "$(WEB_MODE)"

build-editor: ## Build editor mode engine. Optional: GODOT_SRC=/abs/path/to/godot
	$(BUILDCTL_TOOL_CMD) install && $(BUILDCTL_ENGINE_BUILD_CMD) --target editor

build-desktop: ## Build desktop engine. Optional: GODOT_SRC=/abs/path/to/godot
	$(BUILDCTL_TOOL_CMD) install
	$(BUILDCTL_ENGINE_BUILD_CMD) --target template
	$(BUILDCTL_RUNTIME_CMD) export-pack

build-web: ## Build web engine template. Optional: GODOT_SRC=/abs/path/to/godot
	$(BUILDCTL_WORKFLOW_CMD) build-web --mode normal

build-web-worker: ## Build web worker engine template. Optional: GODOT_SRC=/abs/path/to/godot
	$(BUILDCTL_WORKFLOW_CMD) build-web --mode worker

build-web-minigame: ## Build minigame template. Optional: GODOT_SRC=/abs/path/to/godot
	$(BUILDCTL_WORKFLOW_CMD) build-web --mode minigame

build-web-miniprogram: ## Build miniprogram template. Optional: GODOT_SRC=/abs/path/to/godot
	$(BUILDCTL_WORKFLOW_CMD) build-web --mode miniprogram

build-wasm: ## Build wasm
	$(BUILDCTL_RUNTIME_CMD) build-wasm

build-wasm-opt: ## Build wasm with optimization
	$(BUILDCTL_RUNTIME_CMD) build-wasm --opt

build-android: ## Build android engine. Optional: GODOT_SRC=/abs/path/to/godot
	$(BUILDCTL_TOOL_CMD) install && $(BUILDCTL_ENGINE_BUILD_CMD) --target template --platform android

build-ios: ## Build ios engine. Optional: GODOT_SRC=/abs/path/to/godot
	$(BUILDCTL_TOOL_CMD) install && $(BUILDCTL_ENGINE_BUILD_CMD) --target template --platform ios

install-apk: ## Export and install Android APK. Usage: make install-apk [APK_PROJECT_DIR=tutorial/00-Hello]
	$(BUILDCTL_WORKFLOW_CMD) install-apk --project-dir "$(APK_PROJECT_DIR)"

# ============================================
# Run Commands
# ============================================
list-demos: ## List all demos with index
	$(BUILDCTL_WORKFLOW_CMD) list-demos

editor: ## Open demo in editor: make editor DEMO_INDEX=N
	$(BUILDCTL_WORKFLOW_CMD) run-demo --demo-index "$(DEMO_INDEX)" --mode editor --movie "$(MOVIE)"

run: ## Run demo on PC: make run DEMO_INDEX=N
	$(BUILDCTL_WORKFLOW_CMD) run-demo --demo-index "$(DEMO_INDEX)" --mode run --movie "$(MOVIE)"

run-editor: ## Run demo in editor mode: make run-editor DEMO_INDEX=N
	$(BUILDCTL_WORKFLOW_CMD) run-demo --demo-index "$(DEMO_INDEX)" --mode rune --movie "$(MOVIE)"

run-web: ## Run demo on web: make run-web DEMO_INDEX=N
	$(BUILDCTL_WORKFLOW_CMD) run-demo --demo-index "$(DEMO_INDEX)" --mode web --port "$(PORT)"

run-web-worker: ## Run demo on web: make run-web-worker DEMO_INDEX=N
	$(BUILDCTL_WORKFLOW_CMD) run-demo --demo-index "$(DEMO_INDEX)" --mode web-worker --port "$(PORT)"

stop: ## Stop running processes
	$(BUILDCTL_WORKFLOW_CMD) stop-web

# ============================================
# Utility Commands
# ============================================
format: ## Format Go code
	go fmt ./...

generate: ## Generate code. Optional: GODOT_SRC=/abs/path/to/godot
	cd ./internal/cmd/codegen && GODOT_SRC="$(GODOT_SRC)" go run .
	go generate ./cmd/spxrun/runner
	$(MAKE) format

export-pack: ## Export runtime pck file
	$(BUILDCTL_RUNTIME_CMD) export-pack

export-web: ## Export web engine. Usage: make export-web MODE=normal (MODE: normal|worker|minigame|miniprogram)
	$(BUILDCTL_RUNTIME_CMD) export-web --mode "$(WEB_MODE)"
