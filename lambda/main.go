package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/handler"
	"github.com/mbland/elistman/ops"
)

func buildHandler() (*handler.Handler, error) {
	if cfg, err := config.LoadDefaultConfig(context.TODO()); err != nil {
		return nil, err
	} else if opts, err := handler.GetOptions(os.Getenv); err != nil {
		return nil, err
	} else {
		return handler.NewHandler(
			opts.EmailDomainName,
			opts.EmailSiteTitle,
			&ops.ProdAgent{
				Db:        db.NewDynamoDb(cfg, opts.SubscribersTableName),
				Validator: email.AddressValidatorImpl{},
				Mailer:    email.NewSesMailer(cfg),
			},
			opts.RedirectPaths,
			handler.ResponseTemplate,
		)
	}
}

func main() {
	h, err := buildHandler()
	if err != nil {
		log.Fatalf("Failed to initialize process: %s", err.Error())
	}
	log.SetFlags(0)
	lambda.Start(h.HandleEvent)
}
