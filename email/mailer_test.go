//go:build small_tests || all_tests

package email

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

type TestThrottle struct {
	bulkCapNumToSend     int64
	bulkCapError         error
	pauseBeforeSendCalls int
	pauseBeforeSendError error
}

func (tt *TestThrottle) BulkCapacityAvailable(
	_ context.Context, numToSend int64,
) error {
	tt.bulkCapNumToSend = numToSend
	return tt.bulkCapError
}

func (tt *TestThrottle) PauseBeforeNextSend(_ context.Context) error {
	tt.pauseBeforeSendCalls++
	return tt.pauseBeforeSendError
}

func TestSesMailerBulkCapacityAvailablePassThroughToThrottle(t *testing.T) {
	throttle := &TestThrottle{bulkCapError: ErrBulkSendWouldExceedCapacity}
	mailer := &SesMailer{Throttle: throttle}

	err := mailer.BulkCapacityAvailable(context.Background(), 27)

	assert.Equal(t, int64(27), throttle.bulkCapNumToSend)
	assert.Assert(t, testutils.ErrorIs(err, ErrBulkSendWouldExceedCapacity))
}

func TestSend(t *testing.T) {
	setup := func() (
		testSes *TestSesV2,
		throttle *TestThrottle,
		mailer *SesMailer,
		ctx context.Context) {
		testSes = &TestSesV2{
			sendEmailInput:  &sesv2.SendEmailInput{},
			sendEmailOutput: &sesv2.SendEmailOutput{},
		}
		throttle = &TestThrottle{}
		mailer = &SesMailer{
			Client:    testSes,
			ConfigSet: "config-set-name",
			Throttle:  throttle,
		}
		ctx = context.Background()
		return
	}

	testMsgId := "deadbeef"
	recipient := "subscriber@foo.com"
	testMsg := []byte("raw message")

	t.Run("Succeeds", func(t *testing.T) {
		testSes, throttle, mailer, ctx := setup()
		testSes.sendEmailOutput.MessageId = aws.String(testMsgId)

		msgId, err := mailer.Send(ctx, recipient, testMsg)

		assert.NilError(t, err)
		assert.Equal(t, testMsgId, msgId)
		assert.Equal(t, 1, throttle.pauseBeforeSendCalls)

		input := testSes.sendEmailInput
		assert.Assert(t, input != nil)
		assert.DeepEqual(t, []string{recipient}, input.Destination.ToAddresses)
		assert.Equal(
			t, mailer.ConfigSet, aws.ToString(input.ConfigurationSetName),
		)
		assert.DeepEqual(t, testMsg, input.Content.Raw.Data)
	})

	t.Run("ReturnsErrorThrottleFails", func(t *testing.T) {
		_, throttle, mailer, ctx := setup()
		throttle.pauseBeforeSendError = ErrExceededMax24HourSend

		msgId, err := mailer.Send(ctx, recipient, testMsg)

		assert.Equal(t, "", msgId)
		assert.Assert(t, testutils.ErrorIs(err, ErrExceededMax24HourSend))
		assert.ErrorContains(t, err, "send to "+recipient+" failed")
	})

	t.Run("ReturnsErrorIfSendFails", func(t *testing.T) {
		testSes, throttle, mailer, ctx := setup()
		testSes.sendEmailError = testutils.AwsServerError("SendRawEmail error")

		msgId, err := mailer.Send(ctx, recipient, testMsg)

		assert.Equal(t, "", msgId)
		assert.ErrorContains(t, err, "send to "+recipient+" failed")
		assert.ErrorContains(t, err, "SendRawEmail error")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
		assert.Equal(t, 1, throttle.pauseBeforeSendCalls)
	})
}
