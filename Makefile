
format:
	goimport -e -l .

lint:
	golint ./...

vet:
	go vet ./...

build:
	export GOPATH=$(shell godep path):$(shell echo $$GOPATH) &&\
	go build

tools:
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint
	go get -u github.com/axw/gocov/...
	go get -u github.com/AlekSi/gocov-xml

test:
	export GOPATH=$(shell godep path):$(shell echo $$GOPATH) &&\
	gocov test ./... | gocov-xml > coverage.xml
