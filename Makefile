include version.mk

BRANCH:=$(shell git rev-parse --abbrev-ref HEAD)
COMMIT:=$(shell git describe --tags --long | sed -r 's/[0-9.]+-([0-9]+)-(g[a-f0-9]+)/$(VERSION)+\1.\2/')
ARCH:=$(shell go env GOOS).$(shell go env GOARCH)
APP_VERSION=$(COMMIT).$(BRANCH)

PKGSDIRS=$(shell go list -f '{{.Dir}}' ./... | sed /fissile[/]scripts/d)

print_status = @printf "\033[32;01m==> $(1)\033[0m\n"

.PHONY: all clean format lint vet bindata build test docker-deps reap
all: clean format lint vet bindata build test docker-deps

clean:
	$(call print_status, Cleaning)
	rm -rf build/
	rm -f ./fissile
	rm -f ./scripts/compilation/compilation.go
	rm -f ./scripts/dockerfiles/dockerfiles.go
	rm -f ./scripts/templates/transformations.go

format:
	$(call print_status, Checking format)
	export GOPATH=$(shell godep path):$(GOPATH) && \
		test 0 -eq `echo $(PKGSDIRS) | tr ' ' '\n' | xargs -I '{p}' -n1 sh -c 'goimports -d -e {p}/*.go' | tee /dev/fd/2 | wc -l`

lint:
	$(call print_status, Linting)
	test 0 -eq `echo $(PKGSDIRS) | tr ' ' '\n' | xargs -I '{p}' -n1 golint {p} | tee /dev/fd/2 | wc -l`

vet:
	$(call print_status, Vetting)
	go vet ./...

bindata:
	$(call print_status, Generating .go resource files)
	go-bindata -pkg=compilation -o=./scripts/compilation/compilation.go ./scripts/compilation/*.sh && \
	go-bindata -pkg=dockerfiles -o=./scripts/dockerfiles/dockerfiles.go ./scripts/dockerfiles/Dockerfile-* ./scripts/dockerfiles/monitrc.erb ./scripts/dockerfiles/*.sh ./scripts/dockerfiles/rsyslog_conf.tgz

build: bindata
	$(call print_status, Building)
	export GOPATH=$(shell godep path):$(GOPATH) && \
		go build -ldflags="-X main.version=$(APP_VERSION)"


dist: build
	$(call print_status, Disting)
	tar czf fissile-$(APP_VERSION)-$(ARCH).tgz fissile

docker-deps:
	$(call print_status, Installing Docker Image Dependencies)
	docker pull ubuntu:14.04

tools:
	$(call print_status, Installing Tools)
	go get -u golang.org/x/tools/cmd/vet
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint
	go get -u github.com/AlekSi/gocov-xml
	go get -u github.com/jteeuwen/go-bindata/...
	go get -u github.com/tools/godep

# If this fails, try running 'make bindata' and rerun 'make test'
test:
	$(call print_status, Testing)
	export GOPATH=$(shell godep path):$(GOPATH) &&\
		go test -cover ./...

reap:
	# Remove exited test containers
	docker ps -a --filter=status=exited | awk '/fissile-test-/ {print $$1}' | xargs --no-run-if-empty docker rm
