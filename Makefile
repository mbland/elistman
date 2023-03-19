SHELL := /bin/bash
.POSIX:
.PHONY: all clean test coverage delete build-EmailVerifier

# https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/sam-cli-command-reference-sam-build.html#examples-makefile-identifier
build-EmailVerifier:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
			 -o $(ARTIFACTS_DIR)/main main.go

test:
	go test ./...

coverage:
	go test -coverpkg ./... -covermode=count -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out	

sam-build: template.yml
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
