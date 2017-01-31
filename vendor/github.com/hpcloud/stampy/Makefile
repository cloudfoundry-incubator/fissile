#!/usr/bin/env make

GIT_ROOT:=$(shell git rev-parse --show-toplevel)

.PHONY: all clean format lint vet build test

all: clean format lint vet build test

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

tools:
	${GIT_ROOT}/make/tools

test:
	${GIT_ROOT}/make/test

dist:
	${GIT_ROOT}/make/package
