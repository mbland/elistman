package email

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/mbland/elistman/ops"
)

// Suppressor wraps methods for the [SES account-level suppression list].
//
// [SES account-level suppression list]: https://docs.aws.amazon.com/ses/latest/dg/sending-email-suppression-list.html
type Suppressor interface {
	// IsSuppressed checks whether an email address is on the SES account-level
	// suppression list.
	IsSuppressed(ctx context.Context, email string) (bool, error)

	// Suppress adds an email address to the SES account-level suppression list.
	Suppress(ctx context.Context, email string) error

	// Unsuppress removes an email address from the SES account-level
	// suppression list.
	Unsuppress(ctx context.Context, email string) error
}

type SesSuppressor struct {
	Client SesV2Api
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

func (mailer *SesSuppressor) IsSuppressed(
	ctx context.Context, email string,
) (verdict bool, err error) {
	input := &sesv2.GetSuppressedDestinationInput{EmailAddress: &email}
	var notFoundErr *types.NotFoundException

	if _, err = mailer.Client.GetSuppressedDestination(ctx, input); err == nil {
		verdict = true
	} else if errors.As(err, &notFoundErr) {
		err = nil
	} else {
		const errFmt = "unexpected error while checking if %s suppressed"
		err = ops.AwsError(fmt.Sprintf(errFmt, email), err)
	}
	return
}

func (mailer *SesSuppressor) Suppress(ctx context.Context, email string) error {
	input := &sesv2.PutSuppressedDestinationInput{
		EmailAddress: aws.String(email),
		Reason:       types.SuppressionListReasonBounce,
	}

	_, err := mailer.Client.PutSuppressedDestination(ctx, input)

	if err != nil {
		err = ops.AwsError("failed to suppress "+email, err)
	}
	return err
}

func (mailer *SesSuppressor) Unsuppress(ctx context.Context, email string) error {
	input := &sesv2.DeleteSuppressedDestinationInput{
		EmailAddress: aws.String(email),
	}

	_, err := mailer.Client.DeleteSuppressedDestination(ctx, input)

	if err != nil {
		err = ops.AwsError("failed to unsuppress "+email, err)
	}
	return err
}
