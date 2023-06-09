package testutils

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/smithy-go"
	"gotest.tools/assert"
)

type EndpointResolver struct {
	endpoints map[string]*aws.Endpoint
}

func (r *EndpointResolver) AddEndpoint(service, localHostPort string) {
	r.endpoints[service] = &aws.Endpoint{
		URL:               "http://" + localHostPort,
		HostnameImmutable: true,
		Source:            aws.EndpointSourceCustom,
	}
}

func (r *EndpointResolver) CreateEndpoint(service string) (string, error) {
	localHostPort, err := PickUnusedHostPort()

	if err != nil {
		const errFmt = "could not create local %s endpoint: %s"
		return "", fmt.Errorf(errFmt, service, err)
	}
	r.AddEndpoint(service, localHostPort)
	return localHostPort, nil
}

func (r *EndpointResolver) ResolveEndpoint(
	service, region string, options ...interface{},
) (endpoint aws.Endpoint, err error) {
	if ep, ok := r.endpoints[service]; !ok {
		err = &aws.EndpointNotFoundError{Err: errors.New(service + " (local)")}
	} else {
		endpoint = *ep
	}
	return
}

// Inspired by:
// - https://davidagood.com/dynamodb-local-go/
// - https://github.com/aws/aws-sdk-go-v2/blob/main/config/example_test.go
func AwsConfig() (*aws.Config, *EndpointResolver, error) {
	resolver := &EndpointResolver{map[string]*aws.Endpoint{}}
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
		config.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		err = fmt.Errorf("error loading local AWS configuration: %s", err)
		return nil, nil, err
	}
	return &dbConfig, resolver, nil
}

func AwsServerError(msg string) error {
	return &smithy.GenericAPIError{Message: msg, Fault: smithy.FaultServer}
}

func AssertAwsStringEqual(t *testing.T, expected string, actual *string) {
	t.Helper()
	assert.Equal(t, expected, aws.ToString(actual))
}
