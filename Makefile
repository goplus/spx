# ============================================
# Config
# ============================================
.DEFAULT_GOAL := help

export GODOT_SRC
export SPX_MODULE_SRC

BUILDCTL_BIN := .bin/buildctl$(shell go env GOEXE)
MACOS_GO_TOOLCHAIN := cmd/internal/macos_go_toolchain.sh
# Keep go.sum optional so clean repos without it can still build buildctl.
OPTIONAL_GO_SUM := $(wildcard go.sum)
RUNTIME_LOCK_SNAPSHOTS := $(wildcard internal/release/runtime_locks/*.json)
BUILDCTL_SOURCES := go.mod $(OPTIONAL_GO_SUM) $(MACOS_GO_TOOLCHAIN) internal/release/runtime.lock.json $(RUNTIME_LOCK_SNAPSHOTS) $(shell find cmd internal -type f -name '*.go' ! -name '*_test.go' | LC_ALL=C sort)
BUILDCTL_CMD := $(BUILDCTL_BIN)
BUILDCTL_TARGETS := setup setup-web dev doctor list-demos install clean-assets download download-engine build-editor build-desktop build-web build-wasm build-wasm-opt build-android build-ios install-apk editor template-editor run runnative rune runweb runwebworker export-pack export-web stop
PRIMARY_HELP_TARGETS := setup setup-web dev doctor build-editor build-desktop build-web build-android build-ios list-demos editor template-editor run runnative rune runweb runwebworker format generate help-advanced
LOCKED_GO_VERSION = $(shell python3 -c 'import json; print(json.load(open("internal/release/runtime.lock.json"))["toolchain"]["go"])')
LOCKED_GO_HOST_GOOS := $(shell go env GOHOSTOS)
LOCKED_GO = env GOTOOLCHAIN=go$(LOCKED_GO_VERSION) go
# Only macOS needs the SDK-aware wrapper; keep Linux and Windows on direct Go.
ifeq ($(LOCKED_GO_HOST_GOOS),darwin)
LOCKED_GO = bash "$(CURDIR)/$(MACOS_GO_TOOLCHAIN)" go$(LOCKED_GO_VERSION)
endif

.PHONY: $(BUILDCTL_TARGETS) help help-advanced buildctl format generate generate-bindings generate-runtime bump-release pin-godot pin-godot-unpublished pin-godot-candidate clean-projects validate-download-engine validate-install-web validate-bump-release validate-pin-godot

DEMO_INDEX ?= 3
APK_PROJECT_DIR ?= tutorial/00-Hello

PORT    ?= 8106
MOVIE   ?= false
WEB     ?= 0
GODOT_SHA ?=
GODOT_REF ?=
SPX_VERSION ?=
RUNTIME_VERSION ?=
RUNTIME_ABI ?=
WEB_MODE = $(or $(strip $(MODE)),normal)
VALID_INSTALL_WEB_TRUE_VALUES := 1 true TRUE yes YES on ON
VALID_INSTALL_WEB_FALSE_VALUES := 0 false FALSE no NO off OFF
VALID_INSTALL_WEB_VALUES := $(VALID_INSTALL_WEB_TRUE_VALUES) $(VALID_INSTALL_WEB_FALSE_VALUES)

validate-platform-required = $(if $(strip $(PLATFORM)),,$(error PLATFORM is required. Usage: make download-engine PLATFORM=android|ios|web [MODE=normal|worker|minigame|miniprogram]))
validate-install-web = $(if $(filter $(strip $(WEB)),$(VALID_INSTALL_WEB_VALUES)),,$(error invalid WEB "$(WEB)". Expected one of: $(VALID_INSTALL_WEB_TRUE_VALUES) $(VALID_INSTALL_WEB_FALSE_VALUES)))
install-web-flag = $(if $(filter $(strip $(WEB)),$(VALID_INSTALL_WEB_TRUE_VALUES)),--web)

validate-download-engine:
	$(call validate-platform-required)

validate-install-web:
	$(call validate-install-web)

validate-pin-godot:
	@test -n "$(strip $(GODOT_SHA))" || { echo 'GODOT_SHA is required. Usage: make pin-godot[-unpublished|-candidate] GODOT_SHA=<full-sha> [GODOT_REF=<branch-or-tag>]' >&2; exit 2; }
	@test -z "$(strip $(UNPUBLISHED_RUNTIME))" || { echo 'UNPUBLISHED_RUNTIME was replaced by the pin-godot-unpublished target.' >&2; exit 2; }
	@test -z "$(strip $(GODOT_PREMERGE_CANDIDATE))" || { echo 'GODOT_PREMERGE_CANDIDATE was replaced by the pin-godot-candidate target.' >&2; exit 2; }

validate-bump-release:
	@test -n "$(strip $(SPX_VERSION))" || { echo 'SPX_VERSION is required. Usage: make bump-release SPX_VERSION=v3.x.y RUNTIME_VERSION=x.y.z [RUNTIME_ABI=N]' >&2; exit 2; }
	@test -n "$(strip $(RUNTIME_VERSION))" || { echo 'RUNTIME_VERSION is required. Usage: make bump-release SPX_VERSION=v3.x.y RUNTIME_VERSION=x.y.z [RUNTIME_ABI=N]' >&2; exit 2; }

download-engine: validate-download-engine

$(BUILDCTL_TARGETS): $(BUILDCTL_BIN)

$(BUILDCTL_BIN): $(BUILDCTL_SOURCES)
	@mkdir -p $(dir $@)
	@bash -c 'set -euo pipefail; . "$$1"; configure_macos_go_toolchain; macos_go_toolchain_go_build -o "$$2" ./internal/cmd/buildctl' _ "$(MACOS_GO_TOOLCHAIN)" "$@"

# ============================================
# Help
# ============================================
buildctl: $(BUILDCTL_BIN) ## Build cached buildctl binary at ./.bin/buildctl

bump-release: validate-bump-release ## Advance SPX and create a paired immutable runtime snapshot
	python3 .github/scripts/release_bump.py "$(SPX_VERSION)" "$(RUNTIME_VERSION)"$(if $(strip $(RUNTIME_ABI)), --runtime-abi "$(RUNTIME_ABI)")
	git diff --check

pin-godot: override PIN_GODOT_POLICY :=
pin-godot-unpublished: override PIN_GODOT_POLICY := --unpublished
pin-godot-candidate: override PIN_GODOT_POLICY := --premerge
pin-godot pin-godot-unpublished pin-godot-candidate: validate-pin-godot
	python3 .github/scripts/runtime_lock_snapshot.py pin-godot "$(GODOT_SHA)"$(if $(strip $(GODOT_REF)), --ref "$(GODOT_REF)")$(if $(PIN_GODOT_POLICY), $(PIN_GODOT_POLICY))

pin-godot: ## Strictly pin a Godot commit already contained by the current lock ref

pin-godot-unpublished: ## Replace an unpublished snapshot with a commit already contained by the lock ref

pin-godot-candidate: ## Allow a verified pre-merge candidate for dry-run validation only

help: ## Show common commands
	@echo "Common Make Commands:"
	@echo "================================"
	@for target in $(PRIMARY_HELP_TARGETS); do \
		description=$$(awk -v target="$$target:" '$$1 == target { line = $$0; sub(/^.*## /, "", line); print line; exit }' $(MAKEFILE_LIST)); \
		printf "  make %-25s %s\n" "$$target" "$$description"; \
	done
	@echo ""
	@echo "Variable notes:"
	@echo "  MODE defaults to normal for Web-related targets."
	@echo "  GODOT_SRC defaults to ./godot and is used by:"
	@echo "    dev build-editor build-desktop build-web build-android build-ios"
	@echo "  SPX_MODULE_SRC defaults to ./godot_modules/spx and is used by:"
	@echo "    dev build-editor build-desktop build-web build-android build-ios generate"

help-advanced: ## Show all commands, including low-level targets
	@echo "All Make Commands:"
	@echo "================================"
	@grep -E '^[a-zA-Z0-9._-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  make %-25s %s\n", $$1, $$2}'
	@echo ""
	@echo "Advanced variable notes:"
	@echo "  MODE defaults to normal for Web-related targets."
	@echo "  WEB defaults to 0; truthy values enable web tooling/runtime for 'make install'."
	@echo "  PLATFORM is required by download-engine."
	@echo "  GODOT_REF is optional for pin-godot targets; omitting it retains the current lock ref."
	@echo "  bump-release requires authenticated gh access; current tags must be public and target tags unused."
	@echo "  Use pin-godot-unpublished only after confirming that the current snapshot is unpublished."
	@echo "  pin-godot-candidate permits verified candidate-only pinning; never use it for publication."
	@echo "  Examples:"
	@echo "    make bump-release SPX_VERSION=v3.3.0 RUNTIME_VERSION=2.5.0"
	@echo "    make bump-release SPX_VERSION=v3.4.0 RUNTIME_VERSION=3.0.0 RUNTIME_ABI=3"
	@echo "    make pin-godot GODOT_SHA=<40-sha>"
	@echo "    make pin-godot-unpublished GODOT_SHA=<40-sha>"
	@echo "    make pin-godot-candidate GODOT_SHA=<40-sha>"
	@echo ""
	@echo "Demo targets via index:"
	@i=1; \
	for demo in tutorial/*; do \
		if [ -d "$$demo" ]; then \
			echo "  make run DEMO_INDEX=$$i            # Run interpreted $$demo"; \
			echo "  make runnative DEMO_INDEX=$$i      # Run $$demo"; \
			echo "  make runweb DEMO_INDEX=$$i         # Run web $$demo"; \
			echo "  make runwebworker DEMO_INDEX=$$i   # Run web-worker $$demo"; \
			echo "  make rune DEMO_INDEX=$$i           # Run editor runtime $$demo"; \
			i=$$((i+1)); \
		fi; \
	done

# ============================================
# Primary Commands
# ============================================
setup: ## Set up the host SPX development environment
	$(BUILDCTL_CMD) setup host --published-runtime

setup-web: ## Set up Web assets. Usage: make setup-web [MODE=normal|worker|minigame|miniprogram]
	$(BUILDCTL_CMD) setup web --mode "$(WEB_MODE)"

dev: ## Build the complete local development stack. Usage: make dev [MODE=normal|worker|minigame|miniprogram]
	$(BUILDCTL_CMD) build dev --mode "$(WEB_MODE)"

doctor: ## Validate and print the resolved build configuration
	$(BUILDCTL_CMD) doctor

# ============================================
# Install & Download
# ============================================
install: validate-install-web ## Install SPX command. Usage: make install [WEB=1]
	$(BUILDCTL_CMD) tool install $(call install-web-flag)

clean-assets: ## Remove installed SPX/Godot runtime assets from GOPATH/bin
	$(BUILDCTL_CMD) tool clean-assets

download: ## Download engines
	$(BUILDCTL_CMD) engine download --runtime

download-engine: ## Download engine templates for specific platform. Usage: make download-engine PLATFORM=android|ios|web [MODE=normal|worker|minigame|miniprogram]
	@echo "Downloading engine templates for platform: $(PLATFORM)"
	@set --; \
	if [ -n "$(PLATFORM)" ]; then \
		set -- "$$@" --platform "$(PLATFORM)"; \
	fi; \
	if [ -n "$(MODE)" ]; then \
		set -- "$$@" --mode "$(MODE)"; \
	fi; \
	$(BUILDCTL_CMD) engine download "$$@"


# ============================================
# Build Commands
# ============================================
build-editor: ## Build editor mode engine
	$(BUILDCTL_CMD) build editor

build-desktop: ## Build desktop engine
	$(BUILDCTL_CMD) build desktop

build-web: ## Build web engine template. Usage: make build-web [MODE=normal|worker|minigame|miniprogram]
	$(BUILDCTL_CMD) build web --mode "$(WEB_MODE)"

build-wasm: ## Build wasm
	$(BUILDCTL_CMD) runtime build-wasm

build-wasm-opt: ## Build wasm with optimization
	$(BUILDCTL_CMD) runtime build-wasm --opt

build-android: ## Build android engine
	$(BUILDCTL_CMD) build android

build-ios: ## Build ios engine
	$(BUILDCTL_CMD) build ios

install-apk: ## Export and install Android APK. Usage: make install-apk [APK_PROJECT_DIR=tutorial/00-Hello]
	$(BUILDCTL_CMD) workflow install-apk --project-dir "$(APK_PROJECT_DIR)"

# ============================================
# Run Commands
# ============================================
list-demos: ## List all demos with index
	$(BUILDCTL_CMD) workflow list-demos

editor: ## Open demo in editor: make editor DEMO_INDEX=N
	$(BUILDCTL_CMD) workflow run-demo --demo-index "$(DEMO_INDEX)" --mode editor --movie "$(MOVIE)"

template-editor: ## Open cmd/spx template project in editor: make template-editor
	$(BUILDCTL_CMD) workflow open-template-editor

run: ## Run demo in interpreted mode (spx run): make run DEMO_INDEX=N
	$(BUILDCTL_CMD) workflow run-demo --demo-index "$(DEMO_INDEX)" --mode run --movie "$(MOVIE)"

runnative: ## Run demo on native runtime (spx runnative): make runnative DEMO_INDEX=N
	$(BUILDCTL_CMD) workflow run-demo --demo-index "$(DEMO_INDEX)" --mode runnative --movie "$(MOVIE)"

rune: ## Run demo in editor runtime mode (spx rune): make rune DEMO_INDEX=N
	$(BUILDCTL_CMD) workflow run-demo --demo-index "$(DEMO_INDEX)" --mode rune --movie "$(MOVIE)"

runweb: ## Run demo on web: make runweb DEMO_INDEX=N
	$(BUILDCTL_CMD) workflow run-demo --demo-index "$(DEMO_INDEX)" --mode web --port "$(PORT)"

runwebworker: ## Run demo on web worker: make runwebworker DEMO_INDEX=N
	$(BUILDCTL_CMD) workflow run-demo --demo-index "$(DEMO_INDEX)" --mode web-worker --port "$(PORT)"

stop: ## Stop running processes
	$(BUILDCTL_CMD) workflow stop-web

# ============================================
# Utility Commands
# ============================================
format: ## Format Go code
	$(LOCKED_GO) fmt ./...
	cd ./cmd/ispx && $(LOCKED_GO) fmt ./...

generate: ## Generate all code
	$(MAKE) generate-bindings
	$(MAKE) generate-runtime
	$(MAKE) format

generate-bindings: ## Generate Godot/GDExtension binding code
	cd ./internal/cmd/codegen && $(LOCKED_GO) run .

generate-runtime: ## Generate runtime registration code
	$(LOCKED_GO) generate ./pkg/ispx/...
	$(LOCKED_GO) generate ./cmd/spx/internal/command/...
	cd ./cmd/ispx && $(LOCKED_GO) generate ./...

clean-projects: ## Delete generated artifacts (.temp/, project/, .gdspx_web_server*.pid, go.mod, go.sum, gox.mod) from tutorial/test projects
	@find -P tutorial test \( \
		-type d \( -name '.temp' -o -name 'project' \) -prune -print -o \
		-type f \( -name '.gdspx_web_server*.pid' -o -name 'go.mod' -o -name 'go.sum' -o -name 'gox.mod' -o -name 'xgo_autogen.go' \) -print \
	\) | sort | while IFS= read -r path; do \
		rm -rf "$$path" || exit 1; \
		echo "removed $$path"; \
	done

export-pack: ## Export runtime asset bundle
	$(BUILDCTL_CMD) runtime export-pack

export-web: ## Export web engine. Usage: make export-web MODE=normal (MODE: normal|worker|minigame|miniprogram)
	$(BUILDCTL_CMD) runtime export-web --mode "$(WEB_MODE)"
