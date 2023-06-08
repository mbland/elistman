//go:build medium_tests || contract_tests || all_tests

package email

import (
	"bytes"
	"context"
	"flag"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/mbland/elistman/types"
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

	// Defaults to the configuration set created as part of the elistman-dev
	// stack deployed by the mbland/elistman GitHub CI/CD pipeline. The test
	// will fail if the elistman-dev stack isn't running, or if this value
	// doesn't specify another valid SES configuration set.
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

		client := sesv2.NewFromConfig(cfg)
		maxCap, _ := types.NewCapacity(0.8)
		throttle, err := NewSesThrottle(
			ctx, client, maxCap, time.Sleep, time.Now, time.Minute,
		)

		if err != nil {
			panic("failed to initialize throttle: " + err.Error())
		}

		return &SesMailer{
			Client:    client,
			ConfigSet: configurationSetName,
			Throttle:  throttle,
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

		// This will fail if the SES configuration set specified by the
		// -configSet flag isn't available. Generally, this means it will fail
		// if the elistman-dev stack isn't running.
		_, err := mailer.Send(ctx, "success@simulator.amazonses.com", msg)

		assert.NilError(t, err)
	})
}
