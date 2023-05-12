package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/google/uuid"
	"github.com/mbland/elistman/agent"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/handler"
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
		ConfigSet: opts.ConfigurationSet,
	}
	logger := log.Default()
	h, err = handler.NewHandler(
		opts.EmailDomainName,
		opts.EmailSiteTitle,
		&agent.DecoyAgent{
			SenderAddress: fmt.Sprintf(
				"%s <%s@%s>",
				opts.SenderName,
				opts.SenderUserName,
				opts.EmailDomainName,
			),
			UnsubscribeEmail: opts.UnsubscribeUserName +
				"@" + opts.EmailDomainName,
			ApiBaseUrl: fmt.Sprintf(
				"https://%s/%s/", opts.ApiDomainName, opts.ApiMappingKey,
			),
			NewUid:      uuid.NewUUID,
			CurrentTime: time.Now,
			Db: &db.DynamoDb{
				Client:    dynamodb.NewFromConfig(cfg),
				TableName: opts.SubscribersTableName,
			},
			Validator: &email.ProdAddressValidator{
				Suppressor: &email.SesSuppressor{
					Client: sesv2.NewFromConfig(cfg),
				},
				Resolver: net.DefaultResolver,
			},
			Mailer: sesMailer,
			Log:    logger,
		},
		opts.RedirectPaths,
		handler.ResponseTemplate,
		opts.UnsubscribeUserName,
		sesMailer,
		logger,
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
