package email

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	typesV2 "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

type Mailer interface {
	Send(
		ctx context.Context, recipient string, msg []byte,
	) (messageId string, err error)
}

type Bouncer interface {
	Bounce(
		ctx context.Context,
		emailDomain,
		messageId string,
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

	// Unsuppress removes an email address from the SES account-level
	// suppression list.
	Unsuppress(ctx context.Context, email string) error
}

type SesMailer struct {
	Client    SesApi
	ClientV2  SesV2Api
	ConfigSet string
	Log       *log.Logger
}

type SesApi interface {
	SendRawEmail(
		context.Context, *ses.SendRawEmailInput, ...func(*ses.Options),
	) (*ses.SendRawEmailOutput, error)

	SendBounce(
		context.Context, *ses.SendBounceInput, ...func(*ses.Options),
	) (*ses.SendBounceOutput, error)
}

type SesV2Api interface {
	GetSuppressedDestination(
		context.Context,
		*sesv2.GetSuppressedDestinationInput,
		...func(*sesv2.Options),
	) (*sesv2.GetSuppressedDestinationOutput, error)

	PutSuppressedDestination(
		context.Context,
		*sesv2.PutSuppressedDestinationInput,
		...func(*sesv2.Options),
	) (*sesv2.PutSuppressedDestinationOutput, error)

	DeleteSuppressedDestination(
		context.Context,
		*sesv2.DeleteSuppressedDestinationInput,
		...func(*sesv2.Options),
	) (*sesv2.DeleteSuppressedDestinationOutput, error)
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

// https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-example-functions.html
func (mailer *SesMailer) Bounce(
	ctx context.Context,
	emailDomain,
	messageId string,
	recipients []string,
	timestamp time.Time,
) (bounceMessageId string, err error) {
	sender := "mailer-daemon@" + emailDomain
	recipientInfo := make([]types.BouncedRecipientInfo, len(recipients))
	reportingMta := "dns; " + emailDomain
	arrivalDate := timestamp.Truncate(time.Second)
	explanation := "Unauthenticated email is not accepted due to " +
		"the sending domain's DMARC policy."

	for i, recipient := range recipients {
		recipientInfo[i].Recipient = &recipient
		recipientInfo[i].BounceType = types.BounceTypeContentRejected
	}

	input := &ses.SendBounceInput{
		BounceSender:      &sender,
		OriginalMessageId: &messageId,
		MessageDsn: &types.MessageDsn{
			ReportingMta: &reportingMta,
			ArrivalDate:  &arrivalDate,
		},
		Explanation:              &explanation,
		BouncedRecipientInfoList: recipientInfo,
	}
	var output *ses.SendBounceOutput

	if output, err = mailer.Client.SendBounce(ctx, input); err != nil {
		err = fmt.Errorf("sending bounce failed: %s", err)
	} else {
		bounceMessageId = *output.MessageId
	}
	return
}

func (mailer *SesMailer) IsSuppressed(ctx context.Context, email string) bool {
	input := &sesv2.GetSuppressedDestinationInput{EmailAddress: &email}

	_, err := mailer.ClientV2.GetSuppressedDestination(ctx, input)
	if err == nil {
		return true
	}

	// This method returns only a boolean result, not a boolean and an error.
	// This keeps its usage in ProdAddressValidator.ValidateAddress
	// straightforward, without having that method have to propagate an
	// extremely unlikely error.
	//
	// As a result, if we receive an unexpected error, we'll log it and give the
	// address the benefit of the doubt (for now).
	//
	// See also:
	// - https://aws.github.io/aws-sdk-go-v2/docs/handling-errors/#api-error-responses
	// - https://pkg.go.dev/errors#As
	var notFoundErr *typesV2.NotFoundException
	if !errors.As(err, &notFoundErr) {
		const errFmt = "unexpected error while checking if %s suppressed: %s"
		mailer.Log.Printf(errFmt, email, err)
	}
	return false
}

func (mailer *SesMailer) Suppress(ctx context.Context, email string) error {
	input := &sesv2.PutSuppressedDestinationInput{
		EmailAddress: &email, Reason: typesV2.SuppressionListReasonBounce,
	}

	_, err := mailer.ClientV2.PutSuppressedDestination(ctx, input)

	if err != nil {
		err = fmt.Errorf("failed to suppress %s: %s", email, err)
	}
	return err
}

func (mailer *SesMailer) Unsuppress(ctx context.Context, email string) error {
	input := &sesv2.DeleteSuppressedDestinationInput{EmailAddress: &email}

	_, err := mailer.ClientV2.DeleteSuppressedDestination(ctx, input)

	if err != nil {
		err = fmt.Errorf("failed to unsuppress %s: %s", email, err)
	}
	return err
}
