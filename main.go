package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/mbland/ses-subscription-verifier/handler"
)

func buildHandler() (*handler.LambdaHandler, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return nil, err
	}
	return &handler.LambdaHandler{
		Db: handler.NewDynamoDb(cfg, os.Getenv("DB_TABLE_NAME")),
	}, nil
}

func main() {
	h, err := buildHandler()
	if err != nil {
		log.Fatalf("Failed to initialize process: %s", err.Error())
	}
	lambda.Start(h.HandleRequest)
}
