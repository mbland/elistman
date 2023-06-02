//go:build small_tests || all_tests

package cmd

import (
	"context"
	"encoding/json"

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

type TestEListManFunc struct {
	StackName       string
	CreateFuncError error
	InvokeReq       any
	InvokeResJson   []byte
	InvokeError     error
}

func (l *TestEListManFunc) Invoke(_ context.Context, req, res any) error {
	l.InvokeReq = req

	if l.InvokeError != nil {
		return l.InvokeError
	}
	return json.Unmarshal(l.InvokeResJson, res)
}
