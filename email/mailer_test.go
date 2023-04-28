//go:build small_tests || all_tests

package email

import (
	"context"
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

func newTestMailer() *SesMailer {
	return &SesMailer{
		Client:             &TestSes{},
		ConfigSet:          "config-set",
		SenderAddress:      "Mike <mike@foo.com>",
		UnsubscribeBaseUrl: "https://foo.com/email/",
	}
}

// This function will be replaced by more substantial tests once I begin to
// implement SesMailer.
func TestMailerInitialization(t *testing.T) {
	mailer := newTestMailer()

	assert.Assert(t, mailer != nil)
}
