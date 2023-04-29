//go:build small_tests || all_tests

package email

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/google/uuid"
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
		UnsubscribeEmail:   "unsubscribe@foo.com",
		UnsubscribeBaseUrl: "https://foo.com/email/",
	}
}

var testSubscriber *Subscriber = &Subscriber{
	Email: "subscriber@foo.com",
	Uid:   uuid.MustParse("00000000-1111-2222-3333-444444444444"),
}

func TestBuildMessage(t *testing.T) {
	subject := "This is a test"
	textMsg := "This is only a test."
	htmlMsg := strings.Join(
		[]string{
			`<!DOCTYPE html>`,
			`<html>`,
			`<head><title>This is a test</title></head>`,
			`<body><p>This is only a test.</p></body>`,
			`</html>`,
		},
		"\n",
	)

	t.Run("Succeeds", func(t *testing.T) {
		m := newTestMailer()

		msg, err := m.buildMessage(*testSubscriber, subject, textMsg, htmlMsg)

		assert.NilError(t, err)
		assert.Equal(t, string(msg), "")
	})
}
