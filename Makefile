APP_NAME = kudd

all: clean deps build

clean:
	@echo "--> Cleaning build"
	@rm -rf ./bin ./tar ./pkg

format:
	@echo "--> Formatting source code"
	@go fmt ./...

deps:
	@echo "--> Getting dependencies"
	@gb vendor restore

# test:
# 	@echo "--> Testing application"
# 	@gb test src/kudd/...

build: 
	@echo "--> Building application"
	@gb build ./...
	@mkdir -p bin/`go env GOOS`/`go env GOARCH`
	@mkdir -p tar
	@if [ -e bin/${APP_NAME}-`go env GOOS`-`go env GOARCH` ]; then mv bin/${APP_NAME}-`go env GOOS`-`go env GOARCH` bin/`go env GOOS`/`go env GOARCH`/${APP_NAME}; fi;
	@if [ -e bin/${APP_NAME} ]; then mv bin/${APP_NAME} bin/`go env GOOS`/`go env GOARCH`/${APP_NAME}; fi;
	
package: build
	@echo "-->" Packaging application
	@tar cfz tar/${APP_NAME}-`go env GOOS`-`go env GOARCH`.tgz -C bin/`go env GOOS`/`go env GOARCH` ${APP_NAME}
