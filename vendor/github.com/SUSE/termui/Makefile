NO_COLOR=\033[0m
OK_COLOR=\033[32;01m
ERROR_COLOR=\033[31;01m
WARN_COLOR=\033[33;01m

PKGSDIRS=$(shell go list -f '{{.Dir}}' ./...)

all: clean format lint vet build test

.PHONY: all clean format lint vet build test

clean:
	@printf "$(OK_COLOR)==> Cleaning$(NO_COLOR)\n"

format:
	@printf "$(OK_COLOR)==> Checking format$(NO_COLOR)\n"
	goimports -e -l .
	@printf "$(NO_COLOR)"

lint:
	@printf "$(OK_COLOR)==> Linting$(NO_COLOR)\n"
	@printf $(PKGSDIRS) | tr ' ' '\n' | xargs -I '{p}' -n1 golint {p}
	@printf "$(NO_COLOR)"

vet:
	@printf "$(OK_COLOR)==> Vetting$(NO_COLOR)\n"
	go vet ./...
	@printf "$(NO_COLOR)"

build:
	@printf "$(OK_COLOR)==> Building$(NO_COLOR)\n"
	go build -ldflags="-X main.version $(APP_VERSION)"
	@printf "$(NO_COLOR)"

tools:
	@printf "$(OK_COLOR)==> Installing tools$(NO_COLOR)\n"
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint
	go get -u github.com/axw/gocov/...
	go get -u github.com/AlekSi/gocov-xml
	go get github.com/tools/godep
	@printf "$(NO_COLOR)"

test:
	@printf "$(OK_COLOR)==> Testing$(NO_COLOR)\n"
	gocov test ./... | gocov-xml > coverage.xml
	@printf "$(NO_COLOR)"
