.DEFAULT_GOAL := pc

CURRENT_PATH=$(shell pwd)
.PHONY: engine init initweb fmt gen upload templates cmd cmdweb test setuppack

# Format code
fmt:
	go fmt ./... 

# Build templates
templates:
	./pkg/gdspx/tools/build_engine.sh -a

# download engines 
download:
	./pkg/gdspx/tools/build_engine.sh -e -d

# Build current platform's engine
pc:
	./pkg/gdspx/tools/build_engine.sh -e
# Build current platform's engine template
pcpack: 
	./pkg/gdspx/tools/build_engine.sh
# Build web engine
web: 
	make cmdweb && ./pkg/gdspx/tools/build_engine.sh -p web -e
# Build web engine template
webpack: 
	./pkg/gdspx/tools/build_engine.sh -p web

# Build android engine
android:
	./pkg/gdspx/tools/build_engine.sh -p android

# Build ios engine
ios:
	./pkg/gdspx/tools/build_engine.sh -p ios 
# Generate code
gen:
	cd ./pkg/gdspx/cmd/codegen && go run . && cd $(CURRENT_PATH) && make fmt

# Install gdspx command
cmd:
	cd ./cmd/gox/ && ./install.sh && cd $(CURRENT_PATH) 

cmdweb:
	cd ./cmd/gox/ && ./install.sh --web && cd $(CURRENT_PATH) 

cmdwebopt:
	cd ./cmd/gox/ && ./install.sh --web --opt && cd $(CURRENT_PATH) 

# Release web for builder
releaseweb:
	make cmdwebopt && \
	mkdir -p $(CURRENT_PATH)/.tmp/web
	(cd $(CURRENT_PATH)/.tmp/web && \
	 mkdir -p assets && \
	 echo "{\"map\":{\"width\":480,\"height\":360}}" > assets/index.json && \
	 echo "" > main.spx && \
	 rm -rf ./project/.builds/*web && \
	 spx exportweb && \
	 cd ./project/.builds/web && \
	 rm -f game.zip && \
	 zip -r $(CURRENT_PATH)/spx_web.zip * && \
	 echo "$(CURRENT_PATH)/spx_web.zip has been created")
	rm -rf $(CURRENT_PATH)/.tmp

setuppack:
	mkdir -p $(CURRENT_PATH)/.tmp/web && \
	cp $(CURRENT_PATH)/cmd/gox/template/project/runtime.gdextension.txt "$(GOPATH)/bin/runtime.gdextension"
	(cd $(CURRENT_PATH)/.tmp/web && \
	 mkdir -p assets && \
	 echo "{\"map\":{\"width\":480,\"height\":360}}" > assets/index.json && \
	 echo "" > main.spx && \
	 rm -rf ./project/.builds/*web && \
	 mkdir -p "$(GOPATH)/bin" && \
	 spx export && \
	 TEMP_VERSION=$$(cat "$(CURRENT_PATH)/cmd/gox/template/version") && \
	 OUTPUT_PCK="$(GOPATH)/bin/gdspxrt$$TEMP_VERSION.pck" && \
	 cp ./project/.builds/pc/gdexport.pck "$$OUTPUT_PCK" || true && \
	 cp ./project/.builds/pc/gdexport.app/Contents/Resources/*.pck "$$OUTPUT_PCK" || true && \
	 spx exportwebruntime && \
	 cd ./project/.builds/webi && \
	 zip -r $(GOPATH)/bin/gdspxrt_web$$TEMP_VERSION.zip * && \
	 sleep 1 && \
	 echo "$$OUTPUT_PCK has been created")
	rm -rf $(CURRENT_PATH)/.tmp

test:
	cd test/All && spx run . && cd $(CURRENT_PATH) 

path ?= tutorial/01-Weather
runweb:
	@echo "Killing gdspx_web_server.py if running..."
	@PIDS=$$(pgrep -f gdspx_web_server.py); \
	if [ -n "$$PIDS" ]; then \
		echo "Killing process: $$PIDS"; \
		kill -9 $$PIDS; \
	else \
		echo "No gdspx_web_server.py process found."; \
	fi	
	make cmdweb && cd $(path) && spx clear && spx runweb -serveraddr=":8106" && cd $(CURRENT_PATH) 


init:
	make cmd && make download

%:
	@:
