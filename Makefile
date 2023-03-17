SHELL := /bin/bash
.POSIX:
.PHONY: build test coverage

build:
	GOOS=linux go build

test:
	go test -v ./...

coverage:
	go test -v -coverpkg ./... -covermode=count -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out	