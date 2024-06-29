package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	ltypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/ops"
)

const FunctionArnKey = "EListManFunctionArn"

var AwsConfig aws.Config = ops.MustLoadDefaultAwsConfig()

type DynamoDbFactoryFunc func(tableName string) *db.DynamoDb

func NewDynamoDb(tableName string) *db.DynamoDb {
	return db.NewDynamoDb(AwsConfig, tableName, nil)
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

type CloudFormationClientFactoryFunc func() CloudFormationClient

func NewCloudFormationClient() CloudFormationClient {
	return cloudformation.NewFromConfig(AwsConfig)
}

type EListManFunc interface {
	Invoke(ctx context.Context, request, response any) error
}

type EListManFactoryFunc func(stackName string) (EListManFunc, error)

func (newFunc EListManFactoryFunc) Invoke(
	ctx context.Context,
	stackName string,
	request, response any,
) (err error) {
	var elistmanFunc EListManFunc

	if elistmanFunc, err = newFunc(stackName); err != nil {
		return
	}
	return elistmanFunc.Invoke(ctx, request, response)
}

func NewEListManLambda(stackName string) (EListManFunc, error) {
	cfc := NewCloudFormationClient()
	lc := NewLambdaClient()
	return NewLambda(context.Background(), cfc, lc, stackName)
}

type Lambda struct {
	Client LambdaClient
	Arn    string
}

func NewLambda(
	ctx context.Context,
	cfc CloudFormationClient,
	lc LambdaClient,
	stackName string,
) (l *Lambda, err error) {
	var arn string

	if arn, err = GetLambdaArn(ctx, cfc, stackName); err != nil {
		err = fmt.Errorf("could not create Lambda: %w", err)
		return
	}
	l = &Lambda{Client: lc, Arn: arn}
	return
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

func (l *Lambda) Invoke(
	ctx context.Context, request, response any,
) (err error) {
	input := &lambda.InvokeInput{
		FunctionName: aws.String(l.Arn),
		LogType:      ltypes.LogTypeTail,
	}
	var output *lambda.InvokeOutput

	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/lambda#Client.Invoke
	// https://docs.aws.amazon.com/lambda/latest/dg/invocation-sync.html
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/lambda#InvokeInput
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/lambda#InvokeOutput
	if input.Payload, err = json.Marshal(request); err != nil {
		return fmt.Errorf("failed to marshal Lambda request payload: %w", err)
	} else if output, err = l.Client.Invoke(ctx, input); err != nil {
		return ops.AwsError("error invoking Lambda function", err)
	} else if output.StatusCode != http.StatusOK {
		const errFmt = "received non-200 response from Lambda invocation: %s"
		return fmt.Errorf(errFmt, http.StatusText(int(output.StatusCode)))
	} else if output.FunctionError != nil {
		const errFmt = "error executing Lambda function: %s: %s"
		funcErr := aws.ToString(output.FunctionError)
		return fmt.Errorf(errFmt, funcErr, string(output.Payload))
	} else if err = json.Unmarshal(output.Payload, &response); err != nil {
		const errFmt = "failed to unmarshal Lambda response payload: %s: %s"
		return fmt.Errorf(errFmt, err, string(output.Payload))
	}
	return
}
