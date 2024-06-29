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
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/google/uuid"
	"github.com/mbland/elistman/agent"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/handler"
	"github.com/mbland/elistman/ops"
)

func buildHandler() (h *handler.Handler, err error) {
	var cfg aws.Config
	var opts *handler.Options

	if cfg, err = ops.LoadDefaultAwsConfig(); err != nil {
		return
	} else if opts, err = handler.GetOptions(os.Getenv); err != nil {
		return
	}

	sesv2Client := sesv2.NewFromConfig(cfg)
	throttle, err := email.NewSesThrottle(
		context.Background(),
		sesv2Client,
		opts.MaxBulkSendCapacity,
		time.Sleep,
		time.Now,
		time.Minute, // Could be configurable one day.
	)

	if err != nil {
		return
	}

	suppressor := &email.SesSuppressor{Client: sesv2Client}
	logger := log.Default()

	h, err = handler.NewHandler(
		opts.EmailDomainName,
		opts.EmailSiteTitle,
		&agent.ProdAgent{
			SenderAddress: fmt.Sprintf(
				"%s <%s@%s>",
				opts.SenderName,
				opts.SenderUserName,
				opts.EmailDomainName,
			),
			EmailSiteTitle:  opts.EmailSiteTitle,
			EmailDomainName: opts.EmailDomainName,
			UnsubscribeEmail: opts.UnsubscribeUserName +
				"@" + opts.EmailDomainName,
			UnsubscribeUrl: fmt.Sprintf(
				"https://%s/%s", opts.EmailDomainName, opts.UnsubscribeFormPath,
			),
			ApiBaseUrl: fmt.Sprintf(
				"https://%s/%s", opts.ApiDomainName, opts.ApiMappingKey,
			),
			NewUid:      uuid.NewUUID,
			CurrentTime: time.Now,
			Db:          db.NewDynamoDb(cfg, opts.SubscribersTableName, nil),
			Validator: &email.ProdAddressValidator{
				Suppressor: suppressor,
				Resolver:   net.DefaultResolver,
			},
			Mailer: &email.SesMailer{
				Client:    sesv2Client,
				ConfigSet: opts.ConfigurationSet,
				Throttle:  throttle,
			},
			Suppressor: suppressor,
			Log:        logger,
		},
		opts.RedirectPaths,
		handler.ResponseTemplate,
		opts.UnsubscribeUserName,
		&email.SesBouncer{
			Client: ses.NewFromConfig(cfg),
		},
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
