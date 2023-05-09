package agent

import (
	"context"

	"github.com/google/uuid"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/ops"
)

type SubscriptionAgent interface {
	Subscribe(ctx context.Context, email string) (ops.OperationResult, error)
	Verify(
		ctx context.Context, email string, uid uuid.UUID,
	) (ops.OperationResult, error)
	Unsubscribe(
		ctx context.Context, email string, uid uuid.UUID,
	) (ops.OperationResult, error)
	Remove(ctx context.Context, email string) error
	Restore(ctx context.Context, email string) error
}

type ProdAgent struct {
	SenderAddress      string
	UnsubscribeEmail   string
	UnsubscribeBaseUrl string
	Db                 db.Database
	Validator          email.AddressValidator
	Mailer             email.Mailer
}

func (a *ProdAgent) Subscribe(
	ctx context.Context, email string,
) (ops.OperationResult, error) {
	return ops.VerifyLinkSent, nil
}

func (a *ProdAgent) Verify(
	ctx context.Context, email string, uid uuid.UUID,
) (ops.OperationResult, error) {
	return ops.Subscribed, nil
}

func (a *ProdAgent) Unsubscribe(
	ctx context.Context, email string, uid uuid.UUID,
) (ops.OperationResult, error) {
	return ops.Unsubscribed, nil
}

func (a *ProdAgent) Remove(ctx context.Context, email string) error {
	return nil
}

// This should generate a new UUID as well as remove the user from the
// account-level suppression list if present.
func (a *ProdAgent) Restore(ctx context.Context, email string) error {
	return nil
}
