//go:build small_tests || all_tests

package email

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/mbland/elistman/ops"
	tu "github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

const testUnsubEmail = "unsubscribe@foo.com"
const testUnsubUrl = "https://foo.com/unsubscribe"
const testApiBaseUrl = "https://foo.com/email"
const testUid = "00000000-1111-2222-3333-444444444444"

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
	getSupDestInput     *sesv2.GetSuppressedDestinationInput
	getSupDestOutput    *sesv2.GetSuppressedDestinationOutput
	getSupDestError     error
	putSupDestInput     *sesv2.PutSuppressedDestinationInput
	putSupDestOutput    *sesv2.PutSuppressedDestinationOutput
	putSupDestError     error
	deleteSupDestInput  *sesv2.DeleteSuppressedDestinationInput
	deleteSupDestOutput *sesv2.DeleteSuppressedDestinationOutput
	deleteSupDestError  error
	getAccountInput     *sesv2.GetAccountInput
	getAccountOutput    *sesv2.GetAccountOutput
	getAccountError     error
	sendEmailInput      *sesv2.SendEmailInput
	sendEmailOutput     *sesv2.SendEmailOutput
	sendEmailError      error
}

func (ses *TestSesV2) GetSuppressedDestination(
	_ context.Context,
	input *sesv2.GetSuppressedDestinationInput,
	_ ...func(*sesv2.Options),
) (*sesv2.GetSuppressedDestinationOutput, error) {
	ses.getSupDestInput = input
	return ses.getSupDestOutput, ses.getSupDestError
}

func (ses *TestSesV2) PutSuppressedDestination(
	_ context.Context,
	input *sesv2.PutSuppressedDestinationInput,
	_ ...func(*sesv2.Options),
) (*sesv2.PutSuppressedDestinationOutput, error) {
	ses.putSupDestInput = input
	return ses.putSupDestOutput, ses.putSupDestError
}

func (ses *TestSesV2) DeleteSuppressedDestination(
	_ context.Context,
	input *sesv2.DeleteSuppressedDestinationInput,
	_ ...func(*sesv2.Options),
) (*sesv2.DeleteSuppressedDestinationOutput, error) {
	ses.deleteSupDestInput = input
	return ses.deleteSupDestOutput, ses.deleteSupDestError
}

func (ses *TestSesV2) GetAccount(
	_ context.Context, input *sesv2.GetAccountInput, _ ...func(*sesv2.Options),
) (*sesv2.GetAccountOutput, error) {
	ses.getAccountInput = input
	return ses.getAccountOutput, ses.getAccountError
}

func (ses *TestSesV2) SendEmail(
	_ context.Context, input *sesv2.SendEmailInput, _ ...func(*sesv2.Options),
) (*sesv2.SendEmailOutput, error) {
	ses.sendEmailInput = input
	return ses.sendEmailOutput, ses.sendEmailError
}

type TestSuppressor struct {
	checkedEmail       string
	isSuppressedResult bool
	isSuppressedErr    error
	suppressedEmail    string
	suppressedReason   ops.RemoveReason
	suppressErr        error
	unsuppressedEmail  string
	unsuppressErr      error
}

func (ts *TestSuppressor) IsSuppressed(
	ctx context.Context, email string,
) (bool, error) {
	ts.checkedEmail = email
	return ts.isSuppressedResult, ts.isSuppressedErr
}

func (ts *TestSuppressor) Suppress(
	ctx context.Context, email string, reason ops.RemoveReason) error {
	ts.suppressedEmail = email
	ts.suppressedReason = reason
	return ts.suppressErr
}

func (ts *TestSuppressor) Unsuppress(ctx context.Context, email string) error {
	ts.unsuppressedEmail = email
	return ts.unsuppressErr
}

func TestEmitPreviewMessageFromJson(t *testing.T) {

	t.Run("Succeeds", func(t *testing.T) {
		input := strings.NewReader(ExampleMessageJson)
		output := &strings.Builder{}

		err := EmitPreviewMessageFromJson(input, output)

		assert.NilError(t, err)
		msg, _, pr := tu.ParseMultipartMessageAndBoundary(t, output.String())
		assert.Assert(t, msg != nil)
		textPart := tu.GetNextPartContent(t, pr, "text/plain")
		assert.Assert(t, textPart != "")
		htmlPart := tu.GetNextPartContent(t, pr, "text/html")
		assert.Assert(t, htmlPart != "")
	})

	t.Run("FailsIfInputRaisesError", func(t *testing.T) {
		testErr := errors.New("simulated I/O error")
		input := iotest.ErrReader(testErr)
		output := &strings.Builder{}

		err := EmitPreviewMessageFromJson(input, output)

		assert.Assert(t, tu.ErrorIs(err, testErr))
	})

	t.Run("FailsIfOutputRaisesError", func(t *testing.T) {
		input := strings.NewReader(ExampleMessageJson)
		output := &tu.ErrWriter{
			Buf:     &strings.Builder{},
			ErrorOn: "Hello, World!",
			Err:     errors.New("simulated I/O error"),
		}

		err := EmitPreviewMessageFromJson(input, output)

		assert.Assert(t, tu.ErrorIs(err, output.Err))
	})
}
