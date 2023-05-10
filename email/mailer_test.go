//go:build small_tests || all_tests

package email

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/smithy-go"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

type TestSes struct {
	rawEmailInput  *ses.SendRawEmailInput
	rawEmailOutput *ses.SendRawEmailOutput
	rawEmailErr    error
	bounceInput    *ses.SendBounceInput
	bounceOutput   *ses.SendBounceOutput
	bounceErr      error
}

func (ses *TestSes) SendRawEmail(
	_ context.Context, input *ses.SendRawEmailInput, _ ...func(*ses.Options),
) (*ses.SendRawEmailOutput, error) {
	ses.rawEmailInput = input
	return ses.rawEmailOutput, ses.rawEmailErr
}

func (ses *TestSes) SendBounce(
	_ context.Context, input *ses.SendBounceInput, _ ...func(*ses.Options),
) (*ses.SendBounceOutput, error) {
	ses.bounceInput = input
	return ses.bounceOutput, ses.bounceErr
}

func TestSend(t *testing.T) {
	setup := func() (*TestSes, *SesMailer, context.Context) {
		testSes := &TestSes{
			rawEmailInput:  &ses.SendRawEmailInput{},
			rawEmailOutput: &ses.SendRawEmailOutput{},
		}
		mailer := &SesMailer{Client: testSes, ConfigSet: "config-set-name"}
		return testSes, mailer, context.Background()
	}

	testMsgId := "deadbeef"
	recipient := "subscriber@foo.com"
	testMsg := []byte("raw message")

	t.Run("Succeeds", func(t *testing.T) {
		testSes, mailer, ctx := setup()
		testSes.rawEmailOutput.MessageId = aws.String(testMsgId)

		msgId, err := mailer.Send(ctx, recipient, testMsg)

		assert.NilError(t, err)
		assert.Equal(t, testMsgId, msgId)

		input := testSes.rawEmailInput
		assert.Assert(t, input != nil)
		assert.DeepEqual(t, []string{recipient}, input.Destinations)
		assert.Equal(
			t, mailer.ConfigSet, aws.ToString(input.ConfigurationSetName),
		)
		assert.DeepEqual(t, testMsg, input.RawMessage.Data)
	})

	t.Run("ReturnsErrorIfSendFails", func(t *testing.T) {
		testSes, mailer, ctx := setup()
		testSes.rawEmailErr = &smithy.GenericAPIError{
			Message: "SendRawEmail error", Fault: smithy.FaultServer,
		}
		msgId, err := mailer.Send(ctx, recipient, testMsg)

		assert.Equal(t, "", msgId)
		assert.ErrorContains(t, err, "SendRawEmail error")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})
}

func TestBounce(t *testing.T) {
	setup := func() (*TestSes, *SesMailer, context.Context) {
		testSes := &TestSes{
			bounceInput:  &ses.SendBounceInput{},
			bounceOutput: &ses.SendBounceOutput{},
		}
		mailer := &SesMailer{Client: testSes}
		return testSes, mailer, context.Background()
	}

	emailDomain := "foo.com"
	messageId := "deadbeef"
	recipients := []string{"plugh@foo.com"}
	timestamp, _ := time.Parse(time.RFC1123Z, "Fri, 18 Sep 1970 12:45:00 +0000")

	t.Run("Succeeds", func(t *testing.T) {
		testSes, mailer, ctx := setup()
		testBouncedMessageId := "0123456789"
		testSes.bounceOutput.MessageId = aws.String(testBouncedMessageId)

		bouncedId, err := mailer.Bounce(
			ctx, emailDomain, messageId, recipients, timestamp,
		)

		assert.NilError(t, err)
		assert.Equal(t, testBouncedMessageId, bouncedId)

		input := testSes.bounceInput
		assert.Assert(t, input != nil)
		assert.Equal(t, len(recipients), len(input.BouncedRecipientInfoList))
		bouncedRecipient := input.BouncedRecipientInfoList[0]
		assert.Equal(t, recipients[0], aws.ToString(bouncedRecipient.Recipient))
		assert.Equal(
			t, types.BounceTypeContentRejected, bouncedRecipient.BounceType,
		)
	})

	t.Run("ReturnsErrorIfSendBounceFails", func(t *testing.T) {
		testSes, mailer, ctx := setup()
		testSes.bounceErr = &smithy.GenericAPIError{
			Message: "SendBounce error", Fault: smithy.FaultServer,
		}

		bouncedId, err := mailer.Bounce(
			ctx, emailDomain, messageId, recipients, timestamp,
		)

		assert.Equal(t, "", bouncedId)
		assert.ErrorContains(t, err, "SendBounce error")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})
}
