//go:build small_tests || all_tests

package email

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	typesV2 "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
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

type TestSesV2 struct {
	getInput     *sesv2.GetSuppressedDestinationInput
	getOutput    *sesv2.GetSuppressedDestinationOutput
	getError     error
	putInput     *sesv2.PutSuppressedDestinationInput
	putOutput    *sesv2.PutSuppressedDestinationOutput
	putError     error
	deleteInput  *sesv2.DeleteSuppressedDestinationInput
	deleteOutput *sesv2.DeleteSuppressedDestinationOutput
	deleteError  error
}

func (ses *TestSesV2) GetSuppressedDestination(
	_ context.Context,
	input *sesv2.GetSuppressedDestinationInput,
	_ ...func(*sesv2.Options),
) (*sesv2.GetSuppressedDestinationOutput, error) {
	ses.getInput = input
	return ses.getOutput, ses.getError
}

func (ses *TestSesV2) PutSuppressedDestination(
	_ context.Context,
	input *sesv2.PutSuppressedDestinationInput,
	_ ...func(*sesv2.Options),
) (*sesv2.PutSuppressedDestinationOutput, error) {
	ses.putInput = input
	return ses.putOutput, ses.putError
}

func (ses *TestSesV2) DeleteSuppressedDestination(
	_ context.Context,
	input *sesv2.DeleteSuppressedDestinationInput,
	_ ...func(*sesv2.Options),
) (*sesv2.DeleteSuppressedDestinationOutput, error) {
	ses.deleteInput = input
	return ses.deleteOutput, ses.deleteError
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

func TestIsSuppressed(t *testing.T) {
	setup := func() (*TestSesV2, *testutils.Logs, *SesMailer, context.Context) {
		testSesV2 := &TestSesV2{}
		logs, logger := testutils.NewLogs()
		mailer := &SesMailer{ClientV2: testSesV2, Log: logger}
		return testSesV2, logs, mailer, context.Background()
	}

	t.Run("ReturnsTrueIfSuppressed", func(t *testing.T) {
		_, logs, mailer, ctx := setup()

		assert.Assert(t, mailer.IsSuppressed(ctx, "foo@bar.com"))

		expectedEmptyLogs := ""
		assert.Equal(t, expectedEmptyLogs, logs.Logs())
	})

	t.Run("ReturnsFalse", func(t *testing.T) {
		t.Run("IfNotSuppressed", func(t *testing.T) {
			testSesV2, logs, mailer, ctx := setup()
			// Wrap the following error to make sure the implementation is using
			// errors.As properly, versus a type assertion.
			testSesV2.getError = fmt.Errorf(
				"404: %w", &typesV2.NotFoundException{},
			)

			assert.Assert(t, !mailer.IsSuppressed(ctx, "foo@bar.com"))

			expectedEmptyLogs := ""
			assert.Equal(t, expectedEmptyLogs, logs.Logs())
		})

		t.Run("AndLogsErrorIfUnexpectedError", func(t *testing.T) {
			testSesV2, logs, mailer, ctx := setup()
			testSesV2.getError = errors.New("not a 404")

			assert.Assert(t, !mailer.IsSuppressed(ctx, "foo@bar.com"))

			const expectedLog = "unexpected error while checking if " +
				"foo@bar.com suppressed: not a 404"
			logs.AssertContains(t, expectedLog)
		})
	})
}

func TestSuppress(t *testing.T) {
	setup := func() (*TestSesV2, *SesMailer, context.Context) {
		testSesV2 := &TestSesV2{}
		return testSesV2, &SesMailer{ClientV2: testSesV2}, context.Background()
	}

	t.Run("Succeeds", func(t *testing.T) {
		_, mailer, ctx := setup()

		err := mailer.Suppress(ctx, "foo@bar.com")

		assert.NilError(t, err)
	})

	t.Run("ReturnsAnError", func(t *testing.T) {
		testSesV2, mailer, ctx := setup()
		testSesV2.putError = errors.New("testing")

		err := mailer.Suppress(ctx, "foo@bar.com")

		assert.ErrorContains(t, err, "failed to suppress foo@bar.com: testing")
	})
}

func TestUnsuppress(t *testing.T) {
	setup := func() (*TestSesV2, *SesMailer, context.Context) {
		testSesV2 := &TestSesV2{}
		return testSesV2, &SesMailer{ClientV2: testSesV2}, context.Background()
	}

	t.Run("Succeeds", func(t *testing.T) {
		_, mailer, ctx := setup()

		err := mailer.Unsuppress(ctx, "foo@bar.com")

		assert.NilError(t, err)
	})

	t.Run("ReturnsAnError", func(t *testing.T) {
		testSesV2, mailer, ctx := setup()
		testSesV2.deleteError = errors.New("testing")

		err := mailer.Unsuppress(ctx, "foo@bar.com")

		const expectedErr = "failed to unsuppress foo@bar.com: testing"
		assert.ErrorContains(t, err, expectedErr)
	})
}
