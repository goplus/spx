.DEFAULT_GOAL := help

CURRENT_PATH=$(shell pwd)
.PHONY: init fmt gen cmd wasm wasmopt test download help initdev

# Help target - displays available commands
help:
	@echo "Make Commands:"
	@echo "-------------------"
	@echo "Utility Commands:"
	@echo "  init        - Initialize the user environment"
	@echo "  initdev     - Initialize the development environment"
	@echo "  download    - Download engines"
	@echo "  cmd         - Install spx command"
	@echo "  wasm        - Install spx command and build wasm"
	@echo "  wasmopt     - Install spx command and build wasm with optimization"
	@echo "  fmt         - Format code"
	@echo "  gen         - Generate code"
	@echo ""
	@echo "Build Commands:"
	@echo "  pce         - Build current platform's engine (editor mode)"
	@echo "  pc          - Build current platform's engine template"
	@echo "  web         - Build web engine template"
	@echo "  android     - Build Android engine template"
	@echo "  ios         - Build iOS engine template"
	@echo ""
	@echo "Export Commands:"
	@echo "  exportpack  - Export runtime engine pck file"
	@echo "  exportweb   - Export web engine for xbuilder"
	@echo ""
	@echo "Run Commands:"
	@echo "  run         - Run demo on PC (default: tutorial/01-Weather)"
	@echo "  rune        - Run demo on PC editor mode (default: tutorial/01-Weather)"
	@echo "  runweb      - Run demo on web (default: tutorial/01-Weather)"
	@echo "  runtest     - Run tests"
	@echo ""
	@echo "Usage Examples:"
	@echo "  make pc                      - Build for current platform"
	@echo "  make run path=demos/demo1    - Run specific demo on PC (default: tutorial/01-Weather)"
	@echo "  make runweb path=demos/demo1 - Run specific demo on web (default: tutorial/01-Weather)"
	@echo "  make runtest                 - Run tests"

init:
	chmod +x ./pkg/gdspx/tools/*.sh && \
	echo "===>step1/4: cmd" && make cmd && \
	echo "===>step2/4: download engine" && make download && \
	echo "===>step3/4: prepare dev env" && make exportpack && \
	echo "===>step4/4: prepare web template" && ./pkg/gdspx/tools/make_util.sh extrawebtemplate && \
	echo "===>init done"

initdev:
	chmod +x ./pkg/gdspx/tools/*.sh && \
	echo "===>step1/5: cmd" && make cmd && \
	echo "===>step2/5: wasm" && make wasm && \
	echo "===>step3/5: pce" && make pce && \
	echo "===>step4/5: pc" && make pc && \
	echo "===>step5/5: web" && make web && \
	echo "===>initdev done,use `make run` to run demo"

# Format code	
fmt:
	go fmt ./... 

# Generate code
gen:
	cd ./pkg/gdspx/cmd/codegen && go run . && cd $(CURRENT_PATH) && make fmt

# Download engines 
download:
	make cmd &&\
	./pkg/gdspx/tools/build_engine.sh -e -d 

# Install spx command
cmd:
	cd ./cmd/gox/ && ./install.sh && cd $(CURRENT_PATH) 
# build wasm
wasm:
	cd ./cmd/gox/ && ./install.sh --web && cd $(CURRENT_PATH) &&\
	cp -rf /Users/tjp/projects/robot/spx/cmd/igox/gdspx.wasm /Users/tjp/projects/robot/godot-love-wechat/export/engine/gdspx.wasm 
# build wasm with optimization
wasmopt:
	cd ./cmd/gox/ && ./install.sh --web --opt && cd $(CURRENT_PATH) 

# Build current platform's engine (editor mode)
pce:
	make cmd &&\
	./pkg/gdspx/tools/build_engine.sh -e
# Build current platform's engine template
pc: 
	make cmd &&\
	./pkg/gdspx/tools/build_engine.sh &&\
	./pkg/gdspx/tools/make_util.sh exportpack 

# Build web template
web: 
	./pkg/gdspx/tools/build_engine.sh -p web &&\
	cp -rf /Users/tjp/projects/robot/spx/pkg/gdspx/godot/bin/godot.web.template_debug.wasm32.nothreads.wasm /Users/tjp/projects/robot/godot-love-wechat/export/engine/godot.editor.wasm &&\
	cp -rf /Users/tjp/projects/robot/spx/cmd/igox/gdspx.wasm /Users/tjp/projects/robot/godot-love-wechat/export/engine/gdspx.wasm &&\
	./pkg/gdspx/tools/make_util.sh extrawebtemplate 

# Build android template
android:
	./pkg/gdspx/tools/build_engine.sh -p android

# Build ios template
ios:
	./pkg/gdspx/tools/build_engine.sh -p ios 


# Export runtime pck file
exportpack:
	./pkg/gdspx/tools/make_util.sh exportpack && cd $(CURRENT_PATH) 

# Export web engine for builder
exportweb:
	cd ./cmd/gox/ && ./install.sh --web --opt && cd $(CURRENT_PATH) &&\
	./pkg/gdspx/tools/make_util.sh exportweb && cd $(CURRENT_PATH) 


# Run demos
path ?= tutorial/01-Weather
port ?= 8106
# Run demo on PC editor mode
rune:
	cd  $(path) && spx rune . && cd $(CURRENT_PATH) 

# Run demo on PC (runtime mode)
run:
	cd  $(path) && spx run . && cd $(CURRENT_PATH) 

# Run demo on web
runweb:
	./pkg/gdspx/tools/make_util.sh runweb $(path) $(port) && cd $(CURRENT_PATH)  &&\
	cp -rf tutorial/01-Weather/project/.builds/web/godot.editor.js /Users/tjp/projects/robot/godot-love-wechat/export/js/raw/godot.editor.js

# Run tests
runtest:
	cd test/All && spx run . && cd $(CURRENT_PATH) 

# Default rule for unknown targets
%:
	@echo "Unknown target: $@"
	@echo "Run 'make help' for available commands"
	@exit 1
