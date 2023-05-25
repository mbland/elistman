package email

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

type SesApi interface {
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

	SendEmail(
		context.Context,
		*sesv2.SendEmailInput,
		...func(*sesv2.Options),
	) (*sesv2.SendEmailOutput, error)
}
