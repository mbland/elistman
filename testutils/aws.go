package testutils

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/smithy-go"
	"gotest.tools/assert"
)

type BaseEndpoint string

func createBaseEndpoint() (*BaseEndpoint, error) {
	localHostPort, err := PickUnusedHostPort()
	if err != nil {
		return nil, fmt.Errorf("could not create local base endpoint: %s", err)
	}

	endpoint := BaseEndpoint(localHostPort)
	return &endpoint, nil
}

// Inspired by:
// - https://davidagood.com/dynamodb-local-go/
// - https://github.com/aws/aws-sdk-go-v2/blob/main/config/example_test.go
func AwsConfig() (*aws.Config, *BaseEndpoint, error) {
	baseEndpoint, err := createBaseEndpoint()
	if err != nil {
		return nil, nil, err
	}

	dbConfig, err := config.LoadDefaultConfig(
		context.Background(),
		// From: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     "AKID",
				SecretAccessKey: "SECRET",
				SessionToken:    "SESSION",
				Source:          "example hard coded credentials",
			},
		}),
		config.WithRegion("local"),
	)
	if err != nil {
		err = fmt.Errorf("error loading local AWS configuration: %s", err)
		return nil, nil, err
	}
	return &dbConfig, baseEndpoint, nil
}

func AwsServerError(msg string) error {
	return &smithy.GenericAPIError{Message: msg, Fault: smithy.FaultServer}
}

func AssertAwsStringEqual(t *testing.T, expected string, actual *string) {
	t.Helper()
	assert.Equal(t, expected, aws.ToString(actual))
}
