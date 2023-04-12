package ops

import (
	"github.com/google/uuid"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
)

type SubscriptionAgent interface {
	Subscribe(email string) (OperationResult, error)
	Verify(email string, uid uuid.UUID) (OperationResult, error)
	Unsubscribe(email string, uid uuid.UUID) (OperationResult, error)
	Remove(email string) error
	Restore(email string) error
}

type ProdAgent struct {
	Db        db.Database
	Validator email.AddressValidator
	Mailer    email.Mailer
}

func (a *ProdAgent) Subscribe(email string) (OperationResult, error) {
	return VerifyLinkSent, nil
}

func (a *ProdAgent) Verify(
	email string, uid uuid.UUID,
) (OperationResult, error) {
	return Subscribed, nil
}

func (a *ProdAgent) Unsubscribe(
	email string, uid uuid.UUID) (OperationResult, error,
) {
	return Unsubscribed, nil
}

func (a *ProdAgent) Remove(email string) error {
	return nil
}

// This should generate a new UUID as well as remove the user from the
// account-level suppression list if present.
func (a *ProdAgent) Restore(email string) error {
	return nil
}
