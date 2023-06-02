//go:build small_tests || all_tests

package cmd

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

const (
	TestStackName   = "elistman-test"
	TestFunctionArn = "arn:aws:lambda:us-east-1:0123456789:function:" +
		"elistman-dev-Function-0123456789"
)

var TestStack cftypes.Stack = cftypes.Stack{
	StackName: aws.String(TestStackName),
	Outputs: []cftypes.Output{
		{
			OutputKey:   aws.String(FunctionArnKey),
			OutputValue: aws.String(TestFunctionArn),
		},
	},
}
