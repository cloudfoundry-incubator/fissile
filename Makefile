NO_COLOR=\033[0m
OK_COLOR=\033[32;01m
ERROR_COLOR=\033[31;01m
WARN_COLOR=\033[33;01m
PKGSDIRS=$(shell go list -f '{{.Dir}}' ./... | sed /templates/d)

include version.mk

BUILD:=$(shell echo `whoami`-`git rev-parse --short HEAD`-`date -u +%Y%m%d%H%M%S`)
APP_VERSION=$(VERSION)-$(BUILD)

.PHONY: all clean format lint vet bindata build test

all: clean format lint vet bindata build test

clean:
	@echo "$(OK_COLOR)==> Cleaning$(NO_COLOR)"
	rm -f fissile

format:
	@echo "$(OK_COLOR)==> Checking format$(NO_COLOR)"
	goimports -e -l .
	@echo "$(NO_COLOR)\c"

lint:
	@echo "$(OK_COLOR)==> Linting$(NO_COLOR)"
	@echo $(PKGSDIRS) | tr ' ' '\n' | xargs -I '{p}' -n1 golint {p}
	@echo "$(NO_COLOR)\c"

vet:
	@echo "$(OK_COLOR)==> Vetting$(NO_COLOR)"
	go vet ./...
	@echo "$(NO_COLOR)\c"

bindata:
	@echo "$(OK_COLOR)==> Generating code$(NO_COLOR)"
	go-bindata -pkg=compilation -o=./scripts/compilation/compilation.go ./scripts/compilation/*.sh &&\
	go-bindata -pkg=dockerfiles -o=./scripts/dockerfiles/dockerfiles.go ./scripts/dockerfiles/Dockerfile-* ./scripts/dockerfiles/monitrc.erb ./scripts/dockerfiles/*.sh &&\
	go-bindata -pkg=templates -o=./scripts/templates/transformations.go ./scripts/templates/*.yml
	@echo "$(NO_COLOR)\c"
	
build: bindata
	@echo "$(OK_COLOR)==> Building$(NO_COLOR)"
	export GOPATH=$(shell godep path):$(shell echo $$GOPATH) &&\
	gox -verbose \
	-ldflags="-X main.version $(APP_VERSION) " \
	-os="linux darwin " \
	-arch="amd64" \
	-output="build/{{.OS}}-{{.Arch}}/{{.Dir}}" ./...
	@echo "$(NO_COLOR)\c"

tools:
	@echo "$(OK_COLOR)==> Installing tools$(NO_COLOR)"
	docker pull ubuntu:14.04
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint
	go get -u github.com/axw/gocov/...
	go get -u github.com/AlekSi/gocov-xml
	go get -u github.com/jteeuwen/go-bindata/...
	go get -u github.com/tools/godep
	go get -u github.com/mitchellh/gox
	@echo "$(NO_COLOR)\c"

test:
	@echo "$(OK_COLOR)==> Testing$(NO_COLOR)"
	export GOPATH=$(shell godep path):$(shell echo $$GOPATH) &&\
	gocov test ./... | gocov-xml > coverage.xml
	@echo "$(NO_COLOR)\c"
