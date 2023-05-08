package ops

import (
	"context"

	"github.com/google/uuid"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
)

type DecoyAgent struct {
	SenderAddress      string
	UnsubscribeEmail   string
	UnsubscribeBaseUrl string
	Db                 db.Database
	Validator          email.AddressValidator
	Mailer             email.Mailer
}

func (a *DecoyAgent) Subscribe(
	ctx context.Context, email string,
) (OperationResult, error) {
	return VerifyLinkSent, nil
}

func (a *DecoyAgent) Verify(
	ctx context.Context, email string, uid uuid.UUID,
) (OperationResult, error) {
	return Subscribed, nil
}

func (a *DecoyAgent) Unsubscribe(
	ctx context.Context, email string, uid uuid.UUID,
) (OperationResult, error) {
	return Unsubscribed, nil
}

func (a *DecoyAgent) Remove(ctx context.Context, email string) error {
	return nil
}

// This should generate a new UUID as well as remove the user from the
// account-level suppression list if present.
func (a *DecoyAgent) Restore(ctx context.Context, email string) error {
	return nil
}
