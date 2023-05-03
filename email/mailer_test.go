//go:build small_tests || all_tests

package email

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ses"
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
	ctx context.Context, input *ses.SendRawEmailInput, _ ...func(*ses.Options),
) (*ses.SendRawEmailOutput, error) {
	ses.rawEmailInput = input
	return ses.rawEmailOutput, ses.rawEmailErr
}

func (ses *TestSes) SendBounce(
	ctx context.Context, input *ses.SendBounceInput, _ ...func(*ses.Options),
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
		mailer := &SesMailer{testSes, "config-set-name"}
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
