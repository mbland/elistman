package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/mbland/ses-subscription-verifier/db"
	"github.com/mbland/ses-subscription-verifier/email"
	"github.com/mbland/ses-subscription-verifier/handler"
	"github.com/mbland/ses-subscription-verifier/ops"
)

func buildHandler() (*handler.LambdaHandler, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())

	if err != nil {
		return nil, err
	}

	sh := ops.ProdSubscribeHandler{
		Db:        db.NewDynamoDb(cfg, os.Getenv("DB_TABLE_NAME")),
		Validator: email.AddressValidatorImpl{},
		Mailer:    email.NewSesMailer(cfg),
	}
	vh := ops.ProdVerifyHandler{Db: sh.Db, Mailer: sh.Mailer}
	return &handler.LambdaHandler{SubscribeHandler: sh, VerifyHandler: vh}, nil
}

func main() {
	h, err := buildHandler()
	if err != nil {
		log.Fatalf("Failed to initialize process: %s", err.Error())
	}
	lambda.Start(h.HandleRequest)
}
