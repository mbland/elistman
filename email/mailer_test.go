//go:build small_tests || all_tests

package email

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
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

type TestSesV2 struct {
}

func (ses *TestSesV2) GetSuppressedDestination(
	_ context.Context,
	input *sesv2.GetSuppressedDestinationInput,
	_ ...func(*sesv2.Options),
) (*sesv2.GetSuppressedDestinationOutput, error) {
	return nil, nil
}

func (ses *TestSesV2) PutSuppressedDestination(
	_ context.Context,
	input *sesv2.PutSuppressedDestinationInput,
	_ ...func(*sesv2.Options),
) (*sesv2.PutSuppressedDestinationOutput, error) {
	return nil, nil
}

func TestSend(t *testing.T) {
	setup := func() (*TestSes, *SesMailer, context.Context) {
		testSes := &TestSes{
			rawEmailInput:  &ses.SendRawEmailInput{},
			rawEmailOutput: &ses.SendRawEmailOutput{},
		}
		testSesV2 := &TestSesV2{}
		mailer := &SesMailer{testSes, testSesV2, "config-set-name"}
		return testSes, mailer, context.Background()
	}

	testMsgId := "deadbeef"
	recipient := "subscriber@foo.com"
	testMsg := []byte("raw message")

	t.Run("Succeeds", func(t *testing.T) {
		testSes, mailer, ctx := setup()
		testSes.rawEmailOutput.MessageId = &testMsgId

		msgId, err := mailer.Send(ctx, recipient, testMsg)

		assert.NilError(t, err)
		assert.Equal(t, testMsgId, msgId)

		input := testSes.rawEmailInput
		assert.Assert(t, input != nil)
		assert.DeepEqual(t, []string{recipient}, input.Destinations)
		assert.Equal(t, mailer.ConfigSet, *input.ConfigurationSetName)
		assert.DeepEqual(t, testMsg, input.RawMessage.Data)
	})

	t.Run("ReturnsErrorIfSendFails", func(t *testing.T) {
		testSes, mailer, ctx := setup()
		testSes.rawEmailErr = errors.New("SendRawEmail error")

		msgId, err := mailer.Send(ctx, recipient, testMsg)

		assert.Equal(t, "", msgId)
		assert.Error(t, err, "send failed: SendRawEmail error")
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
		testSes.bounceOutput.MessageId = &testBouncedMessageId

		bouncedId, err := mailer.Bounce(
			ctx, emailDomain, messageId, recipients, timestamp,
		)

		assert.NilError(t, err)
		assert.Equal(t, testBouncedMessageId, bouncedId)

		input := testSes.bounceInput
		assert.Assert(t, input != nil)
		assert.Equal(t, len(recipients), len(input.BouncedRecipientInfoList))
		bouncedRecipient := input.BouncedRecipientInfoList[0]
		assert.Equal(t, recipients[0], *bouncedRecipient.Recipient)
		assert.Equal(
			t, types.BounceTypeContentRejected, bouncedRecipient.BounceType,
		)
	})

	t.Run("ReturnsErrorIfSendBounceFails", func(t *testing.T) {
		testSes, mailer, ctx := setup()
		testSes.bounceErr = errors.New("SendBounce error")

		bouncedId, err := mailer.Bounce(
			ctx, emailDomain, messageId, recipients, timestamp,
		)

		assert.Equal(t, "", bouncedId)
		assert.Error(t, err, "sending bounce failed: SendBounce error")
	})
}
