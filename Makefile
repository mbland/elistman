SHELL := /bin/bash
.POSIX:
.PHONY: build test coverage run-local

build: main
	GOOS=linux go build -o main main.go

test:
	go test ./...

coverage:
	go test -coverpkg ./... -covermode=count -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out	

run-local: build
	sam local start-api --port 8080