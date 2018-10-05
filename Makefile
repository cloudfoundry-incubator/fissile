#!/usr/bin/env make

ifeq ($(GIT_ROOT),)


GIT_ROOT:=$(shell git rev-parse --show-toplevel)
endif

.PHONY: all clean format lint vet bindata build test docker-deps reap dist

all: clean format lint vet test build

clean:
	${GIT_ROOT}/make/clean

format:
	${GIT_ROOT}/make/format

lint:
	${GIT_ROOT}/make/lint

vet:
	${GIT_ROOT}/make/vet

build:
	${GIT_ROOT}/make/build

dist:
	${GIT_ROOT}/make/package

release:
	${GIT_ROOT}/make/release

docker-deps:
	${GIT_ROOT}/make/docker-deps

tools:
	${GIT_ROOT}/make/tools

show-versions:
	make/show-versions

test:
	${GIT_ROOT}/make/test

reap:
	${GIT_ROOT}/make/reap

markdown:
	${GIT_ROOT}/make/generate-markdown
