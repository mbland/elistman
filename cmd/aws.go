package cmd

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/ops"
)

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
