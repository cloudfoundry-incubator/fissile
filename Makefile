#!/usr/bin/env make

VERSION:=$(shell cat VERSION)
VERSION_OFFSET:=$(shell git describe --tags --long | sed -r 's/[0-9.]+-([0-9]+)-(g[a-f0-9]+)/\1.\2/')
BRANCH:=$(shell (git describe --all --exact-match HEAD 2>/dev/null || echo HEAD) | sed 's@.*/@@')
ARCH:=$(shell go env GOOS).$(shell go env GOARCH)
APP_VERSION=$(VERSION)+$(VERSION_OFFSET).$(BRANCH)

PKGSDIRS=$(shell go list -f '{{.Dir}}' ./... | sed /fissile[/]scripts/d)

print_status = @printf "\033[32;01m==> $(1)\033[0m\n"
GIT_ROOT:=$(shell git rev-parse --show-toplevel)

.PHONY: all clean format lint vet bindata build test docker-deps reap
all: clean format lint vet bindata build test docker-deps

clean:
	${GIT_ROOT}/make/clean

format:
	${GIT_ROOT}/make/format

lint:
	${GIT_ROOT}/make/lint

vet:
	${GIT_ROOT}/make/vet

bindata:
	${GIT_ROOT}/make/bindata

build:
	${GIT_ROOT}/make/build

dist:
	${GIT_ROOT}/make/package

docker-deps:
	${GIT_ROOT}/make/docker-deps

tools:
	${GIT_ROOT}/make/tools

# If this fails, try running 'make bindata' and rerun 'make test'
test:
	${GIT_ROOT}/make/test

reap:
	${GIT_ROOT}/make/reap
