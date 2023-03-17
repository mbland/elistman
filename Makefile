SHELL := /bin/bash
.POSIX:
.PHONY: test coverage

build-local:
	GOOS=linux go build -o main main.go

build-prod:
	GOOS=linux GOARCH=amd64 go build -o main main.go
	sam build

test:
	go test ./...

coverage:
	go test -coverpkg ./... -covermode=count -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out	

run-local: build-local
	sam local start-api --port 8080
