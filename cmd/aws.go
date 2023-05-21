package cmd

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/mbland/elistman/db"
	"github.com/spf13/cobra"
)

type AwsConfigFactoryFunc func() (aws.Config, error)

func NewAwsCommandFunc(
	loadAwsConfig AwsConfigFactoryFunc,
	runFunc func(aws.Config, *cobra.Command, []string) error,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) (err error) {
		var cfg aws.Config
		if cfg, err = loadAwsConfig(); err != nil {
			return
		}
		return runFunc(cfg, cmd, args)
	}
}

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
