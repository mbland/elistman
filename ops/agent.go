package ops

import (
	"context"

	"github.com/google/uuid"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
)

type SubscriptionAgent interface {
	Subscribe(ctx context.Context, email string) (OperationResult, error)
	Verify(
		ctx context.Context, email string, uid uuid.UUID,
	) (OperationResult, error)
	Unsubscribe(
		ctx context.Context, email string, uid uuid.UUID,
	) (OperationResult, error)
	Remove(ctx context.Context, email string) error
	Restore(ctx context.Context, email string) error
}

type ProdAgent struct {
	Db        db.Database
	Validator email.AddressValidator
	Mailer    email.Mailer
}

func (a *ProdAgent) Subscribe(
	ctx context.Context, email string,
) (OperationResult, error) {
	return VerifyLinkSent, nil
}

func (a *ProdAgent) Verify(
	ctx context.Context, email string, uid uuid.UUID,
) (OperationResult, error) {
	return Subscribed, nil
}

func (a *ProdAgent) Unsubscribe(
	ctx context.Context, email string, uid uuid.UUID,
) (OperationResult, error) {
	return Unsubscribed, nil
}

func (a *ProdAgent) Remove(ctx context.Context, email string) error {
	return nil
}

// This should generate a new UUID as well as remove the user from the
// account-level suppression list if present.
func (a *ProdAgent) Restore(ctx context.Context, email string) error {
	return nil
}
