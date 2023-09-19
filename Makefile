SHELL := /bin/bash
.POSIX:
.PHONY: all clean \
	delete deploy run-local sam-build \
	coverage test contract-tests-aws medium-tests small-tests static-checks\
	build-Function

# https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/sam-cli-command-reference-sam-build.html#examples-makefile-identifier
# https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html
# https://github.com/aws-samples/sessions-with-aws-sam/tree/master/go-al2
# https://aws.amazon.com/blogs/compute/migrating-aws-lambda-functions-from-the-go1-x-runtime-to-the-custom-runtime-on-amazon-linux-2/
build-Function:
	GOEXPERIMENT=loopvar GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build \
		-ldflags="-s -w" -tags lambda.norpc \
		-o $(ARTIFACTS_DIR)/bootstrap lambda/main.go

static-checks:
	go vet -tags=all_tests ./...
	go run honnef.co/go/tools/cmd/staticcheck -tags=all_tests ./...
	GOEXPERIMENT=loopvar go build -tags=all_tests ./...

small-tests:
	GOEXPERIMENT=loopvar go test -tags=small_tests ./...

medium-tests:
	GOEXPERIMENT=loopvar go test -tags=medium_tests -count=1 ./...

contract-tests-aws:
	GOEXPERIMENT=loopvar go test -tags=contract_tests -count=1 ./db -args -awsdb

test: static-checks small-tests medium-tests contract-tests-aws

coverage:
	GOEXPERIMENT=loopvar go test -covermode=count -coverprofile=coverage.out \
	  -tags=small_tests,coverage_tests ./...
	GOEXPERIMENT=loopvar go tool cover -html=coverage.out	

sam-build: template.yml
	sam validate
	sam validate --lint
	sam build

run-local: sam-build deploy.env
	bin/sam-with-env.sh deploy.env local start-api --port 8080

deploy: sam-build deploy.env
	bin/sam-with-env.sh deploy.env deploy

delete: template.yml
	bin/sam-with-env.sh deploy.env delete

clean:
	rm -rf coverage.out .aws-sam
	go clean
	go clean -testcache

all: sam-build
