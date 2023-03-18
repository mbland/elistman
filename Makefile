SHELL := /bin/bash
.POSIX:
.PHONY: all clean test coverage delete

sam-build: template.yml
	sam build

main-prod:
	GOOS=linux GOARCH=amd64 go build -o main-prod main.go

main-local:
	GOOS=linux go build -o main-local main.go

test:
	go test ./...

coverage:
	go test -coverpkg ./... -covermode=count -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out	

run-local: main-local sam-build
	sam local start-api --port 8080

deploy: main-prod sam-build deploy.env
	bin/deploy.sh deploy.env

delete:
	sam delete

clean:
	rm -f coverage.out main-local main-prod
	go clean
	go clean -testcache

all: main-prod
