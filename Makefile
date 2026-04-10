# ============================================
# Config
# ============================================
.DEFAULT_GOAL := help
.PHONY: help buildctl list-demos prepare-host prepare-web prepare-full prepare-all build-dev install clean-assets download download-engine build-editor build-desktop build-web build-wasm build-wasm-opt build-android build-ios install-apk editor run rune run-web run-web-worker format generate clean-projects export-pack export-web stop validate-web-mode validate-download-engine

export GODOT_SRC

BUILDCTL_BIN := .bin/buildctl$(shell go env GOEXE)
# Keep go.sum optional so clean repos without it can still build buildctl.
OPTIONAL_GO_SUM := $(wildcard go.sum)
BUILDCTL_SOURCES := go.mod $(OPTIONAL_GO_SUM) $(shell find cmd internal -type f -name '*.go' ! -name '*_test.go' | LC_ALL=C sort)
BUILDCTL_CMD := $(BUILDCTL_BIN)
BUILDCTL_TOOL_CMD := $(BUILDCTL_CMD) tool
BUILDCTL_ENGINE_DOWNLOAD_CMD := $(BUILDCTL_CMD) engine download
BUILDCTL_ENGINE_BUILD_CMD := $(BUILDCTL_CMD) engine build
BUILDCTL_RUNTIME_CMD := $(BUILDCTL_CMD) runtime
BUILDCTL_WORKFLOW_CMD := $(BUILDCTL_CMD) workflow
BUILDCTL_TARGETS := list-demos prepare-host prepare-web prepare-full build-dev install clean-assets download download-engine build-editor build-desktop build-web build-wasm build-wasm-opt build-android build-ios install-apk editor run rune run-web run-web-worker export-pack export-web stop

DEMO_INDEX ?= 3
APK_PROJECT_DIR ?= tutorial/00-Hello

PORT    ?= 8106
MOVIE   ?= false
WEB_MODE = $(or $(strip $(MODE)),normal)
VALID_WEB_MODES := normal worker minigame miniprogram
VALID_ENGINE_PLATFORMS := android ios web linux windows macos

validate-web-mode = $(if $(filter $(WEB_MODE),$(VALID_WEB_MODES)),,$(error invalid WEB_MODE/MODE "$(WEB_MODE)". Expected one of: $(VALID_WEB_MODES)))
validate-platform-required = $(if $(strip $(PLATFORM)),,$(error PLATFORM is required. Usage: make download-engine PLATFORM=android|ios|web [MODE=normal|worker|minigame|miniprogram]))
validate-engine-platform = $(if $(filter $(PLATFORM),$(VALID_ENGINE_PLATFORMS)),,$(error invalid PLATFORM "$(PLATFORM)". Expected one of: $(VALID_ENGINE_PLATFORMS)))
validate-download-engine-mode = $(if $(filter web,$(PLATFORM)),$(call validate-web-mode),$(if $(strip $(MODE)),$(error MODE is only supported when PLATFORM=web),))

validate-web-mode:
	$(call validate-web-mode)

validate-download-engine:
	$(call validate-platform-required)
	$(call validate-engine-platform)
	$(call validate-download-engine-mode)

prepare-full prepare-web build-dev build-web export-web: validate-web-mode
download-engine: validate-download-engine

$(BUILDCTL_TARGETS): $(BUILDCTL_BIN)

$(BUILDCTL_BIN): $(BUILDCTL_SOURCES)
	@mkdir -p $(dir $@)
	go build -o $@ ./internal/cmd/buildctl

# ============================================
# Help
# ============================================
buildctl: $(BUILDCTL_BIN) ## Build cached buildctl binary at ./.bin/buildctl

help: ## Show available commands
	@echo "Make Commands:"
	@echo "================================"
	@grep -E '^[a-zA-Z0-9._-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  make %-25s %s\n", $$1, $$2}'
	@echo ""
	@echo "Variable notes:"
	@echo "  MODE defaults to normal for Web-related targets."
	@echo "  GODOT_SRC defaults to ./godot and is used by:"
	@echo "    build-dev build-editor build-desktop build-web build-android build-ios generate"
	@echo ""
	@echo "Demo targets via index:"
	@i=1; \
	for demo in tutorial/*; do \
		if [ -d "$$demo" ]; then \
			echo "  make run DEMO_INDEX=$$i            # Run $$demo"; \
			echo "  make run-web DEMO_INDEX=$$i        # Run web $$demo"; \
			echo "  make run-web-worker DEMO_INDEX=$$i # Run web-worker $$demo"; \
			echo "  make rune DEMO_INDEX=$$i           # Run editor runtime $$demo"; \
			i=$$((i+1)); \
		fi; \
	done

# ============================================
# Prepare Commands
# ============================================
prepare-full: ## Prepare host assets plus web export assets. Usage: make prepare-full [MODE=normal|worker|minigame|miniprogram]
	$(BUILDCTL_CMD) prepare --setup-mode full --web-mode "$(WEB_MODE)"

prepare-all: prepare-full ## Alias of prepare-full. Prepare host assets plus web export assets. Usage: make prepare-all [MODE=normal|worker|minigame|miniprogram]

prepare-host: ## Prepare host assets, including editor/runtime files
	$(BUILDCTL_CMD) prepare --setup-mode runtime

prepare-web: ## Prepare web export assets for MODE, including the host editor required by exporttemplateweb. Usage: make prepare-web MODE=normal (MODE: normal|worker|minigame|miniprogram)
	$(BUILDCTL_CMD) prepare --setup-mode web --web-mode "$(WEB_MODE)"

# ============================================
# Install & Download
# ============================================
install: ## Install SPX command
	$(BUILDCTL_TOOL_CMD) install

clean-assets: ## Remove installed SPX/Godot runtime assets from GOPATH/bin
	$(BUILDCTL_TOOL_CMD) clean-assets

download: ## Download engines
	$(BUILDCTL_ENGINE_DOWNLOAD_CMD) --runtime

download-engine: ## Download engine templates for specific platform. Usage: make download-engine PLATFORM=android|ios|web [MODE=normal|worker|minigame|miniprogram]
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
build-dev: ## Build the full local development environment. Usage: make build-dev [MODE=normal|worker|minigame|miniprogram]
	$(BUILDCTL_WORKFLOW_CMD) build-dev --web-mode "$(WEB_MODE)"

build-editor: ## Build editor mode engine
	$(BUILDCTL_TOOL_CMD) install && $(BUILDCTL_ENGINE_BUILD_CMD) --target editor

build-desktop: ## Build desktop engine
	$(BUILDCTL_TOOL_CMD) install
	$(BUILDCTL_ENGINE_BUILD_CMD) --target template
	$(BUILDCTL_RUNTIME_CMD) export-pack

build-web: ## Build web engine template. Usage: make build-web [MODE=normal|worker|minigame|miniprogram]
	$(BUILDCTL_WORKFLOW_CMD) build-web --mode "$(WEB_MODE)"

build-wasm: ## Build wasm
	$(BUILDCTL_RUNTIME_CMD) build-wasm

build-wasm-opt: ## Build wasm with optimization
	$(BUILDCTL_RUNTIME_CMD) build-wasm --opt

build-android: ## Build android engine
	$(BUILDCTL_TOOL_CMD) install && $(BUILDCTL_ENGINE_BUILD_CMD) --target template --platform android

build-ios: ## Build ios engine
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

rune: ## Run demo in editor runtime mode: make rune DEMO_INDEX=N
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

generate: ## Generate code
	cd ./internal/cmd/codegen && GODOT_SRC="$(GODOT_SRC)" go run .
	go generate ./cmd/spxrunner/runner
	$(MAKE) format

clean-projects: ## Delete generated files from all tutorial/test projects
	@find tutorial test \( \
		-type d \( -name '.temp' -o -name 'project' \) -o \
		-type f \( -name 'go.mod' -o -name 'go.sum' -o -name 'gox.mod' \) \
	\) | sort | while read -r path; do \
		rm -rf "$$path" || exit 1; \
		echo "removed $$path"; \
	done

export-pack: ## Export runtime pck file
	$(BUILDCTL_RUNTIME_CMD) export-pack

export-web: ## Export web engine. Usage: make export-web MODE=normal (MODE: normal|worker|minigame|miniprogram)
	$(BUILDCTL_RUNTIME_CMD) export-web --mode "$(WEB_MODE)"
