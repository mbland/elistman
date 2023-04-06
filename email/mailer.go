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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
)

type Mailer interface {
	Send(
		toAddr string,
		fromAddr string,
		subject string,
		textMsg string,
		htmlMsg string,
	) error
}

type Bouncer interface {
	Bounce(
		emailDomain string, recipients []string, timestamp time.Time,
	) (string, error)
}

type SesMailer struct {
	Client *ses.Client
}

func NewSesMailer(awsConfig aws.Config) *SesMailer {
	return &SesMailer{Client: ses.NewFromConfig(awsConfig)}
}

func (mailer *SesMailer) Send(
	toAddr string,
	fromAddr string,
	subject string,
	textMsg string,
	htmlMsg string,
) error {
	return nil
}

func (mailer *SesMailer) Bounce(
	emailDomain string, recipients []string, timestamp time.Time,
) (string, error) {
	// https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-example-functions.html
	// https://docs.aws.amazon.com/sdk-for-go/api/service/ses/#SES.SendBounce
	// https://docs.aws.amazon.com/ses/latest/APIReference/API_SendBounce.html
	// https://docs.aws.amazon.com/ses/latest/APIReference/API_MessageDsn.html
	// https://docs.aws.amazon.com/sdk-for-go/api/service/ses/#MessageDsn
	// https://docs.aws.amazon.com/sdk-for-go/api/service/ses/sesiface/
	return "fake bounce message ID", nil
}
