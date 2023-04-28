package email

// This will take cues from the defunct gomail package to build a message:
// - https://github.com/go-gomail/gomail
//
// then send it via the RawMessage field of the SES SendEmail API:
// - https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_SendEmail.html
//
// This must be a raw message because we're setting our own List-Unsubscribe and
// List-Unsubscribe-Post HTTP headers:
// - https://www.litmus.com/blog/the-ultimate-guide-to-list-unsubscribe/
// - https://mailtrap.io/blog/list-unsubscribe-header/
// - https://certified-senders.org/wp-content/uploads/2017/07/CSA_one-click_list-unsubscribe.pdf
// - https://www.postmastery.com/list-unsubscribe-header-critical-for-sustained-email-delivery/
// - https://www.ietf.org/rfc/rfc2369.txt
// - https://www.rfc-editor.org/rfc/rfc2369
// - https://www.ietf.org/rfc/rfc8058.txt
// - https://www.rfc-editor.org/rfc/rfc8058
//
// As it turns out, all the necessary building blocks are in the Go standard
// library:
// - https://pkg.go.dev/mime
// - https://pkg.go.dev/mime/multipart
// - https://pkg.go.dev/mime/quotedprintable
//
// See also:
// - https://en.wikipedia.org/wiki/MIME

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

type Mailer interface {
	Send(
		ctx context.Context, toAddr, subject, textMsg, htmlMsg string,
	) (string, error)
}

type Bouncer interface {
	Bounce(
		ctx context.Context,
		emailDomain string,
		recipients []string,
		timestamp time.Time,
	) (string, error)
}

// Suppressor wraps methods for the [SES account-level suppression list].
//
// [SES account-level suppression list]: https://docs.aws.amazon.com/ses/latest/dg/sending-email-suppression-list.html
type Suppressor interface {
	// IsSuppressed checks whether an email address is on the SES account-level
	// suppression list.
	IsSuppressed(ctx context.Context, email string) bool

	// Suppress adds an email address to the SES account-level suppression list.
	Suppress(ctx context.Context, email string) error
}

type SesMailer struct {
	Client             SesApi
	ConfigSet          string
	SenderAddress      string
	UnsubscribeBaseUrl string
}

type SesApi interface {
	SendRawEmail(
		context.Context, *ses.SendRawEmailInput, ...func(*ses.Options),
	) (*ses.SendRawEmailOutput, error)

	SendBounce(
		context.Context, *ses.SendBounceInput, ...func(*ses.Options),
	) (*ses.SendBounceOutput, error)
}

func (mailer *SesMailer) Send(
	ctx context.Context, toAddr, subject, textMsg, htmlMsg string,
) (messageId string, err error) {
	msg, err := buildMessage(
		toAddr, mailer.SenderAddress, subject, textMsg, htmlMsg,
	)
	if err != nil {
		return
	}

	sesMsg := &ses.SendRawEmailInput{
		Destinations:         []string{toAddr},
		ConfigurationSetName: &mailer.ConfigSet,
		RawMessage:           &types.RawMessage{Data: msg},
	}
	var output *ses.SendRawEmailOutput

	if output, err = mailer.Client.SendRawEmail(ctx, sesMsg); err != nil {
		err = fmt.Errorf("send failed: %s", err)
	} else {
		messageId = *output.MessageId
	}
	return
}

func buildMessage(
	toAddr, fromAddr, subject, textMsg, htmlMsg string,
) (msg []byte, err error) {
	return
}

func (mailer *SesMailer) Bounce(
	ctx context.Context,
	emailDomain string,
	recipients []string,
	timestamp time.Time,
) (bounceMessageId string, err error) {
	// https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-example-functions.html
	// https://docs.aws.amazon.com/sdk-for-go/api/service/ses/#SES.SendBounce
	// https://docs.aws.amazon.com/ses/latest/APIReference/API_SendBounce.html
	// https://docs.aws.amazon.com/ses/latest/APIReference/API_MessageDsn.html
	// https://docs.aws.amazon.com/sdk-for-go/api/service/ses/#MessageDsn
	// https://docs.aws.amazon.com/sdk-for-go/api/service/ses/sesiface/
	bounceMessageId = "fake bounce message ID"
	return
}

func (mailer *SesMailer) IsSuppressed(ctx context.Context, email string) bool {
	return false
}

func (mailer *SesMailer) Suppress(ctx context.Context, email string) error {
	return nil
}
