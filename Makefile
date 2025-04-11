.DEFAULT_GOAL := pc

CURRENT_PATH=$(shell pwd)
.PHONY: engine init initweb fmt gen upload templates cmd cmdweb test

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

test:
	cd test/All && spx run . && cd $(CURRENT_PATH) 

init:
	make cmd && make download

%:
	@:
