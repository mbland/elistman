//go:build medium_tests || contract_tests || all_tests

package email

import (
	"bytes"
	"context"
	"flag"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"gotest.tools/assert"
)

var fromAddress string
var configurationSetName string

func init() {
	flag.StringVar(
		&fromAddress,
		"fromAddr",
		"testing@mike-bland.com",
		"From: address, must be one you've verified for your AWS account",
	)

	// This may not strictly be necessary, but we set it to the name of the
	// configuration set created as part of the elistman-dev pipeline.
	flag.StringVar(
		&configurationSetName,
		"configSet",
		"elistman-dev",
		"Name of the Configuration Set to apply when sending",
	)
}

// https://docs.aws.amazon.com/ses/latest/dg/send-an-email-from-console.html
func TestSendWithLiveSes(t *testing.T) {
	setup := func() (*SesMailer, context.Context) {
		ctx := context.Background()
		cfg, err := config.LoadDefaultConfig(ctx)

		if err != nil {
			panic("failed to load AWS config: " + err.Error())
		}

		return &SesMailer{
			Client:    sesv2.NewFromConfig(cfg),
			ConfigSet: configurationSetName,
		}, ctx
	}

	t.Run("Success", func(t *testing.T) {
		mailer, ctx := setup()
		msgBuf := bytes.Buffer{}
		msgBuf.WriteString("From: " + fromAddress + "\r\n")
		msgBuf.WriteString("To: success@simulator.amazonses.com\r\n")
		msgBuf.WriteString("Subject: Successful mailer.Send test\r\n\r\n")
		msgBuf.WriteString("This should work just fine.\r\n")
		msg := msgBuf.Bytes()

		_, err := mailer.Send(ctx, "success@simulator.amazonses.com", msg)

		assert.NilError(t, err)
	})
}
