SHELL := /bin/bash
.POSIX:
.PHONY: all clean test coverage delete

main: FORCE
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o main main.go

FORCE: ;

test:
	go test ./...

coverage:
	go test -coverpkg ./... -covermode=count -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out	

sam-build: main template.yml
	sam build

run-local: sam-build
	sam local start-api --port 8080

deploy: sam-build deploy.env
	bin/deploy.sh deploy.env

delete:
	sam delete

clean:
	rm -rf coverage.out main .aws-sam
	go clean
	go clean -testcache

all: sam-build
