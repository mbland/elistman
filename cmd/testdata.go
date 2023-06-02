//go:build small_tests || all_tests

package cmd

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

const (
	TestStackName   = "elistman-test"
	TestFunctionArn = "arn:aws:lambda:us-east-1:0123456789:function:" +
		"elistman-dev-Function-0123456789"
)

var TestStack types.Stack = types.Stack{
	StackName: aws.String(TestStackName),
	Outputs: []types.Output{
		{
			OutputKey:   aws.String(FunctionArnKey),
			OutputValue: aws.String(TestFunctionArn),
		},
	},
}
