package cmd

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/ops"
)

var AwsConfig aws.Config = ops.MustLoadDefaultAwsConfig()

type DynamoDbFactoryFunc func(cfg aws.Config, tableName string) *db.DynamoDb

func NewDynamoDb(cfg aws.Config, tableName string) *db.DynamoDb {
	return &db.DynamoDb{
		Client: dynamodb.NewFromConfig(cfg), TableName: tableName,
	}
}

type LambdaClient interface {
	Invoke(
		context.Context,
		*lambda.InvokeInput,
		...func(*lambda.Options),
	) (*lambda.InvokeOutput, error)
}

type LambdaClientFactoryFunc func(cfg aws.Config) LambdaClient

func NewLambdaClient(cfg aws.Config) LambdaClient {
	return lambda.NewFromConfig(cfg)
}
