package email

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

type Mailer interface {
	Send(
		ctx context.Context, recipient string, msg []byte,
	) (messageId string, err error)
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
	Client    SesApi
	ConfigSet string
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
	ctx context.Context, recipient string, msg []byte,
) (messageId string, err error) {
	sesMsg := &ses.SendRawEmailInput{
		Destinations:         []string{recipient},
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
