//go:build small_tests || medium_tests || all_tests

package email

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestBounce(t *testing.T) {
	setup := func() (*TestSes, *SesBouncer, context.Context) {
		testSes := &TestSes{
			bounceInput:  &ses.SendBounceInput{},
			bounceOutput: &ses.SendBounceOutput{},
		}
		bouncer := &SesBouncer{Client: testSes}
		return testSes, bouncer, context.Background()
	}

	emailDomain := "foo.com"
	messageId := "deadbeef"
	recipients := []string{"plugh@foo.com"}
	timestamp, _ := time.Parse(time.RFC1123Z, "Fri, 18 Sep 1970 12:45:00 +0000")

	t.Run("Succeeds", func(t *testing.T) {
		testSes, bounce, ctx := setup()
		testBouncedMessageId := "0123456789"
		testSes.bounceOutput.MessageId = aws.String(testBouncedMessageId)

		bouncedId, err := bounce.Bounce(
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
		testSes, bouncer, ctx := setup()
		testSes.bounceErr = testutils.AwsServerError("SendBounce error")

		bouncedId, err := bouncer.Bounce(
			ctx, emailDomain, messageId, recipients, timestamp,
		)

		assert.Equal(t, "", bouncedId)
		assert.ErrorContains(t, err, "SendBounce error")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})
}
