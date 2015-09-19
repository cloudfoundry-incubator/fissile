
format:
	goimport -e -l .

lint:
	golint ./...

vet:
	go vet ./...

bindata:
	go-bindata -pkg=compilation -o=./baseos/compilation/compilation.go ./baseos/compilation/*.sh

build: bindata
	export GOPATH=$(shell godep path):$(shell echo $$GOPATH) &&\
	go build

tools:
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint
	go get -u github.com/axw/gocov/...
	go get -u github.com/AlekSi/gocov-xml
	go get -u github.com/jteeuwen/go-bindata/...

test:
	export GOPATH=$(shell godep path):$(shell echo $$GOPATH) &&\
	gocov test ./... | gocov-xml > coverage.xml
