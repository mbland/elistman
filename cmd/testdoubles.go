//go:build small_tests || all_tests

package cmd

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

type TestLambdaClient struct {
	InvokeInput  *lambda.InvokeInput
	InvokeOutput *lambda.InvokeOutput
	InvokeError  error
}

func NewTestLambdaClient() *TestLambdaClient {
	return &TestLambdaClient{InvokeOutput: &lambda.InvokeOutput{}}
}

func (tlc *TestLambdaClient) Invoke(
	_ context.Context, input *lambda.InvokeInput, _ ...func(*lambda.Options),
) (*lambda.InvokeOutput, error) {
	tlc.InvokeInput = input
	return tlc.InvokeOutput, tlc.InvokeError
}
