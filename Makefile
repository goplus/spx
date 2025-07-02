.DEFAULT_GOAL := help

CURRENT_PATH=$(shell pwd)
.PHONY: init fmt gen cmd wasm wasmopt test download help initdev
.PHONY: setup setup-dev install build-wasm build-wasm-opt format generate stop
.PHONY: build-editor build-desktop build-web build-minigame build-miniprogram build-web-worker build-android build-ios
.PHONY: export-pack export-web run-editor run-web test run-minigame run-minigame-opt run-miniprogram serve

# Run demos
path ?= tutorial/01-Weather
port ?= 8106
mode ?= ""

# Help target - displays available commands
help:
	@echo "Make Commands:"
	@echo "============================="
	@echo "Setup Commands:"
	@echo "  setup            - Initialize the user environment"
	@echo "  setup-dev        - Initialize the development environment"
	@echo "  download         - Download engines"
	@echo "  install          - Install spx command"
	@echo "  build-wasm       - Install spx command and build wasm"
	@echo "  build-wasm-opt   - Install spx command and build wasm with optimization"
	@echo "  format           - Format code"
	@echo "  generate         - Generate code"
	@echo "  stop             - Stop running processes"
	@echo ""
	@echo "Build Commands:"
	@echo "  build-editor     - Build current platform's engine (editor mode)"
	@echo "  build-desktop    - Build current platform's engine template"
	@echo "  build-web        - Build web engine template"
	@echo "  build-minigame   - Build web minigame template"
	@echo "  build-miniprogram - Build web miniprogram template"
	@echo "  build-web-worker - Build web worker template"
	@echo "  build-android    - Build Android engine template"
	@echo "  build-ios        - Build iOS engine template"
	@echo ""
	@echo "Export Commands:"
	@echo "  export-pack      - Export runtime engine pck file"
	@echo "  export-web       - Export web engine for builder"
	@echo ""
	@echo "Run Commands:"
	@echo "  run              - Run demo on PC runtime mode (default: tutorial/06-worker)"
	@echo "  run-editor       - Run demo on PC editor mode (default: tutorial/06-worker)"
	@echo "  run-web          - Run demo on web (default: tutorial/06-worker)"
	@echo "  run-web-worker   - Run demo on web worker mode (default: tutorial/06-worker)"
	@echo "  test             - Run tests"
	@echo "  run-minigame     - Run minigame"
	@echo "  run-minigame-opt - Run minigame with optimization"
	@echo "  run-miniprogram  - Run miniprogram"
	@echo "  serve            - Run web server"
	@echo ""
	@echo "Parameters:"
	@echo "  path=<dir>       - Specify demo path (default: tutorial/06-worker)"
	@echo "  port=<num>       - Specify port number (default: 8106)"
	@echo "  mode=<mode>      - Specify mode (default: empty)"
	@echo ""
	@echo "Usage Examples:"
	@echo "  make build-desktop                    - Build for current platform"
	@echo "  make build-minigame                   - Build web minigame template"
	@echo "  make build-miniprogram                - Build web miniprogram template"
	@echo "  make run path=test/Hello              - Run specific demo on PC"
	@echo "  make run-web path=test/Hello port=8080 - Run specific demo on web with custom port"
	@echo "  make run-web-worker path=test/Hello   - Run specific demo in web worker mode"
	@echo "  make test                             - Run tests"
	@echo "  make stop                             - Stop all running processes"

# ============================================================================
# Setup Commands
# ============================================================================

setup: init
init:
	chmod +x ./pkg/gdspx/tools/*.sh && \
	echo "===>step1/4: cmd" && make install && \
	echo "===>step2/4: download engine" && make download && \
	echo "===>step3/4: prepare dev env" && make export-pack && \
	echo "===>step4/4: prepare web template" && ./pkg/gdspx/tools/make_util.sh extrawebtemplate && \
	echo "===>init done"

setup-dev: initdev
initdev:
	chmod +x ./pkg/gdspx/tools/*.sh && \
	echo "===>step1/5: cmd" && make install && \
	echo "===>step2/5: wasm" && make build-wasm && \
	echo "===>step3/5: pce" && make build-editor && \
	echo "===>step4/5: pc" && make build-desktop && \
	echo "===>step5/5: web" && make build-web && \
	echo "===>initdev done,use `make run` to run demo"

# Download engines 
download:
	make install &&\
	./pkg/gdspx/tools/build_engine.sh -e -d 

# Install spx command
install: cmd
cmd:
	cd ./cmd/gox/ && ./install.sh && cd $(CURRENT_PATH) 

# Build wasm
build-wasm: wasm
wasm:
	cd ./cmd/gox/ && ./install.sh --web && cd $(CURRENT_PATH) 

# Build wasm with optimization
build-wasm-opt: wasmopt
wasmopt:
	cd ./cmd/gox/ && ./install.sh --web --opt && cd $(CURRENT_PATH) 
	./pkg/gdspx/tools/make_util.sh compresswasm

# Format code	
format: fmt
fmt:
	go fmt ./... 

# Generate code
generate: gen
gen:
	cd ./pkg/gdspx/cmd/codegen && go run . && cd $(CURRENT_PATH) && make format

# ============================================================================
# Build Commands
# ============================================================================

# Build current platform's engine (editor mode)
build-editor: pce
pce:
	make install &&\
	./pkg/gdspx/tools/build_engine.sh -e

# Build current platform's engine template
build-desktop: pc
pc: 
	make install &&\
	./pkg/gdspx/tools/build_engine.sh &&\
	./pkg/gdspx/tools/make_util.sh exportpack 

# Build web template
build-web: web
web: 
	./pkg/gdspx/tools/build_engine.sh -p web &&\
	./pkg/gdspx/tools/make_util.sh extrawebtemplate normal

build-web-worker: webworker
webworker: 
	./pkg/gdspx/tools/build_engine.sh -p web -m worker &&\
	./pkg/gdspx/tools/make_util.sh extrawebtemplate worker

build-minigame: minigame
minigame: 
	./pkg/gdspx/tools/build_engine.sh -p web -m minigame &&\
	./pkg/gdspx/tools/make_util.sh extrawebtemplate minigame

build-miniprogram: miniprogram
miniprogram: 
	./pkg/gdspx/tools/build_engine.sh -p web -m miniprogram &&\
	./pkg/gdspx/tools/make_util.sh extrawebtemplate miniprogram

# Build android template
build-android: android
android:
	./pkg/gdspx/tools/build_engine.sh -p android

# Build ios template
build-ios: ios
ios:
	./pkg/gdspx/tools/build_engine.sh -p ios 

# ============================================================================
# Export Commands
# ============================================================================

# Export runtime pck file
export-pack: exportpack
exportpack:
	./pkg/gdspx/tools/make_util.sh exportpack && cd $(CURRENT_PATH) 

# Export web engine for builder
export-web: exportweb
exportweb:
	cd ./cmd/gox/ && ./install.sh --web --opt && cd $(CURRENT_PATH) &&\
	./pkg/gdspx/tools/make_util.sh exportweb && cd $(CURRENT_PATH) 

# ============================================================================
# Run Commands
# ============================================================================

# Run demo on PC editor mode
run-editor: rune
rune:
	cd  $(path) && spx rune . && cd $(CURRENT_PATH) 

# Run demo on PC (runtime mode)
run:
	cd  $(path) && spx run . && cd $(CURRENT_PATH) 

# Run tests
test: runtest
runtest:
	cd test/All && spx run . && cd $(CURRENT_PATH) 
	
# Run demo on web
run-web: runweb
runweb:
	make stop &&\
	make build-wasm &&\
	cd $(path) && spx clear && spx runweb -serveraddr=":$(port)" && cd $(CURRENT_PATH)

# Run demo on web worker
run-web-worker: runwebworker
runwebworker:
	make stop &&\
	make build-wasm &&\
	cd $(path) && spx clear && spx runweb -serveraddr=":$(port)" -mode=worker && cd $(CURRENT_PATH)

# Run minigame
run-minigame: runmg
runmg:
	make install &&\
	cd  $(path) && spx exportminigame -build=fast && cd $(CURRENT_PATH) 
	
# Run minigame with optimization
run-minigame-opt: runmgopt
runmgopt:
	make build-wasm-opt &&\
	cd  $(path) && spx exportminigame && cd $(CURRENT_PATH) 

# Run miniprogram
run-miniprogram: runmp
runmp:
	make install &&\
	cd  $(path) && spx exportminiprogram && cd $(CURRENT_PATH) &&\
	make serve

# Run web server
serve: runwebserver
runwebserver:
	make stop && cd $(CURRENT_PATH) &&\
	cd $(path) && python3 ./project/.godot/gdspx_web_server.py -r "../.builds/web" -p $(port)

# Stop running processes
stop: stopwebserver
stopwebserver:
	@echo "Stopping running processes..."
	@if [ "$$OS" = "Windows_NT" ] || [[ "$$(uname -s 2>/dev/null)" == MINGW* ]]; then \
		echo "Windows environment detected, !!WARN: using taskkill to kill all python processes"; \
		taskkill /F /FI "IMAGENAME eq python.exe" 2>/dev/null || true; \
		taskkill /F /FI "IMAGENAME eq pythonw.exe" 2>/dev/null || true; \
		taskkill /F /FI "IMAGENAME eq python3.exe" 2>/dev/null || true; \
	elif command -v pgrep > /dev/null; then \
		echo "Unix/Linux environment detected, using pgrep and kill"; \
		PIDS=$$(pgrep -f gdspx_web_server.py); \
		if [ -n "$$PIDS" ]; then \
			echo "Killing process: $$PIDS"; \
			kill -9 $$PIDS; \
		else \
			echo "No gdspx_web_server.py process found."; \
		fi \
	else \
		echo "Neither taskkill nor pgrep available, skipping process killing"; \
	fi
	@echo "Process stopping completed."

# Default rule for unknown targets
%:
	@echo "Unknown target: $@"
	@echo "Run 'make help' for available commands"
	@exit 1
