package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/ops"
)

const FunctionArnKey = "EListManFunctionArn"

var AwsConfig aws.Config = ops.MustLoadDefaultAwsConfig()

type DynamoDbFactoryFunc func(tableName string) *db.DynamoDb

func NewDynamoDb(tableName string) *db.DynamoDb {
	return db.NewDynamoDb(AwsConfig, tableName)
}

type LambdaClient interface {
	Invoke(
		context.Context,
		*lambda.InvokeInput,
		...func(*lambda.Options),
	) (*lambda.InvokeOutput, error)
}

type LambdaClientFactoryFunc func() LambdaClient

func NewLambdaClient() LambdaClient {
	return lambda.NewFromConfig(AwsConfig)
}

type CloudFormationClient interface {
	DescribeStacks(
		context.Context,
		*cloudformation.DescribeStacksInput,
		...func(*cloudformation.Options),
	) (*cloudformation.DescribeStacksOutput, error)
}

func GetLambdaArn(
	ctx context.Context, cfc CloudFormationClient, stackName string,
) (arn string, err error) {
	input := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}
	var output *cloudformation.DescribeStacksOutput

	if output, err = cfc.DescribeStacks(ctx, input); err != nil {
		err = ops.AwsError("failed to get Lambda ARN for "+stackName, err)
		return
	} else if len(output.Stacks) == 0 {
		err = errors.New("stack not found: " + stackName)
		return
	}

	stack := &output.Stacks[0]
	for i := range stack.Outputs {
		output := &stack.Outputs[i]

		if aws.ToString(output.OutputKey) == FunctionArnKey {
			arn = aws.ToString(output.OutputValue)
			return
		}
	}
	const errFmt = `stack "%s" doesn't contain output key "%s"`
	err = fmt.Errorf(errFmt, stackName, FunctionArnKey)
	return
}
