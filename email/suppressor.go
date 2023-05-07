package email

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

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

type SesSuppressor struct {
	Client SesV2Api
	Log    *log.Logger
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

func (mailer *SesSuppressor) IsSuppressed(ctx context.Context, email string) bool {
	input := &sesv2.GetSuppressedDestinationInput{EmailAddress: &email}

	_, err := mailer.Client.GetSuppressedDestination(ctx, input)
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
	var notFoundErr *types.NotFoundException
	if !errors.As(err, &notFoundErr) {
		const errFmt = "unexpected error while checking if %s suppressed: %s"
		mailer.Log.Printf(errFmt, email, err)
	}
	return false
}

func (mailer *SesSuppressor) Suppress(ctx context.Context, email string) error {
	input := &sesv2.PutSuppressedDestinationInput{
		EmailAddress: &email, Reason: types.SuppressionListReasonBounce,
	}

	_, err := mailer.Client.PutSuppressedDestination(ctx, input)

	if err != nil {
		err = fmt.Errorf("failed to suppress %s: %s", email, err)
	}
	return err
}

func (mailer *SesSuppressor) Unsuppress(ctx context.Context, email string) error {
	input := &sesv2.DeleteSuppressedDestinationInput{EmailAddress: &email}

	_, err := mailer.Client.DeleteSuppressedDestination(ctx, input)

	if err != nil {
		err = fmt.Errorf("failed to unsuppress %s: %s", email, err)
	}
	return err
}
