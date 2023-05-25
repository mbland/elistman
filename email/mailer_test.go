//go:build small_tests || all_tests

package email

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

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
		testSes.rawEmailErr = testutils.AwsServerError("SendRawEmail error")
		msgId, err := mailer.Send(ctx, recipient, testMsg)

		assert.Equal(t, "", msgId)
		assert.ErrorContains(t, err, "send to "+recipient+" failed")
		assert.ErrorContains(t, err, "SendRawEmail error")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})
}
