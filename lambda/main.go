package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/handler"
	"github.com/mbland/elistman/ops"
)

func buildHandler() (h *handler.Handler, err error) {
	var cfg aws.Config
	var opts *handler.Options

	if cfg, err = config.LoadDefaultConfig(context.Background()); err != nil {
		return
	} else if opts, err = handler.GetOptions(os.Getenv); err != nil {
		return
	}

	sesMailer := &email.SesMailer{
		Client:    ses.NewFromConfig(cfg),
		ClientV2:  sesv2.NewFromConfig(cfg),
		ConfigSet: opts.ConfigurationSet,
		Log:       log.Default(),
	}
	h, err = handler.NewHandler(
		opts.EmailDomainName,
		opts.EmailSiteTitle,
		&ops.ProdAgent{
			SenderAddress: fmt.Sprintf(
				"%s <%s@%s>",
				opts.SenderName,
				opts.SenderUserName,
				opts.EmailDomainName,
			),
			UnsubscribeEmail: opts.UnsubscribeUserName +
				"@" + opts.EmailDomainName,
			UnsubscribeBaseUrl: fmt.Sprintf(
				"https://%s/%s/", opts.ApiDomainName, opts.ApiMappingKey,
			),
			Db: &db.DynamoDb{
				Client:    dynamodb.NewFromConfig(cfg),
				TableName: opts.SubscribersTableName,
			},
			Validator: &email.ProdAddressValidator{
				Suppressor: sesMailer,
				Resolver:   net.DefaultResolver,
			},
			Mailer: sesMailer,
		},
		opts.RedirectPaths,
		handler.ResponseTemplate,
		opts.UnsubscribeUserName,
		sesMailer,
		log.Default(),
	)
	return
}

func main() {
	// Disable standard logger flags. The CloudWatch logs show that the Lambda
	// runtime already adds a timestamp at the beginning of every log line
	// emitted by the function.
	log.SetFlags(0)

	if h, err := buildHandler(); err != nil {
		log.Fatalf("Failed to initialize process: %s", err.Error())
	} else {
		lambda.Start(h.HandleEvent)
	}
}
