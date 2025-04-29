.DEFAULT_GOAL := pc

CURRENT_PATH=$(shell pwd)
.PHONY: engine init initweb fmt gen upload templates cmd cmdweb test

# Format code
fmt:
	go fmt ./... 

# Build templates
templates:
	./gdspx/tools/build_engine.sh -a

# download engines 
download:
	./gdspx/tools/build_engine.sh -e -d

# Build current platform's engine
pc:
	./gdspx/tools/build_engine.sh -e
# Generate code
gen:
	cd ./gdspx/cmd/codegen && go run . && cd $(CURRENT_PATH) && make fmt

# Install gdspx command
cmd:
	go mod tidy &&cd cmd/gox/ && ./install.bat && cd $(CURRENT_PATH) && \
	cd ./gdspx/cmd/gdspx/ && go install . &&  cd $(CURRENT_PATH) 

test:
	cd ./tutorial/05-Animation && spx run . && cd $(CURRENT_PATH) 

init:
	make cmd && make pc

%:
	@:
