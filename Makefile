SHELL := /bin/bash
.POSIX:
.PHONY: all clean delete deploy run-local sam-build coverage test build-EListMan

# https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/sam-cli-command-reference-sam-build.html#examples-makefile-identifier
# https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html
# https://github.com/aws-samples/sessions-with-aws-sam/tree/master/go-al2
build-EListMan:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
			 -o $(ARTIFACTS_DIR)/main lambda/main.go

test:
	go vet ./...
	staticcheck ./...
	go test ./...

coverage:
	go test -covermode=count -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out	

sam-build: template.yml
	sam validate
	sam validate --lint
	sam build

run-local: sam-build deploy.env
	bin/sam-with-env.sh deploy.env local start-api --port 8080

deploy: sam-build deploy.env
	bin/sam-with-env.sh deploy.env deploy

delete: template.yml deploy.env
	sam delete

clean:
	rm -rf coverage.out .aws-sam
	go clean
	go clean -testcache

all: sam-build
