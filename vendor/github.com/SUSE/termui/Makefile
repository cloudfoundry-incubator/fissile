NO_COLOR=\033[0m
OK_COLOR=\033[32;01m
ERROR_COLOR=\033[31;01m
WARN_COLOR=\033[33;01m

PKGSDIRS=$(shell go list -f '{{.Dir}}' ./...)

all: clean format lint vet build test

.PHONY: all clean format lint vet build test

clean:
	@echo "$(OK_COLOR)==> Cleaning$(NO_COLOR)"

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

build:
	@echo "$(OK_COLOR)==> Building$(NO_COLOR)"
	export GOPATH=$(shell godep path):$(shell echo $$GOPATH) && \
		go build -ldflags="-X main.version $(APP_VERSION)"
	@echo "$(NO_COLOR)\c"

tools:
	@echo "$(OK_COLOR)==> Installing tools$(NO_COLOR)"
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint
	go get -u github.com/axw/gocov/...
	go get -u github.com/AlekSi/gocov-xml
	go get github.com/tools/godep
	@echo "$(NO_COLOR)\c"

test:
	@echo "$(OK_COLOR)==> Testing$(NO_COLOR)"
	export GOPATH=$(shell godep path):$(shell echo $$GOPATH) &&\
	gocov test ./... | gocov-xml > coverage.xml
	@echo "$(NO_COLOR)\c"
