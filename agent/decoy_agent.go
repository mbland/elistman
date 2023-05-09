package agent

import (
	"context"

	"github.com/google/uuid"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/ops"
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
) (ops.OperationResult, error) {
	return ops.VerifyLinkSent, nil
}

func (a *DecoyAgent) Verify(
	ctx context.Context, email string, uid uuid.UUID,
) (ops.OperationResult, error) {
	return ops.Subscribed, nil
}

func (a *DecoyAgent) Unsubscribe(
	ctx context.Context, email string, uid uuid.UUID,
) (ops.OperationResult, error) {
	return ops.Unsubscribed, nil
}

func (a *DecoyAgent) Remove(ctx context.Context, email string) error {
	return nil
}

// This should generate a new UUID as well as remove the user from the
// account-level suppression list if present.
func (a *DecoyAgent) Restore(ctx context.Context, email string) error {
	return nil
}
