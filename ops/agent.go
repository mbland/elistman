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
}

type ProdAgent struct {
	Db        db.Database
	Validator email.AddressValidator
	Mailer    email.Mailer
}

func (a *ProdAgent) Subscribe(email string) (OperationResult, error) {
	return Invalid, nil
}

func (a *ProdAgent) Verify(
	email string, uid uuid.UUID,
) (OperationResult, error) {
	return Invalid, nil
}

func (a *ProdAgent) Unsubscribe(
	email string, uid uuid.UUID) (OperationResult, error,
) {
	return Invalid, nil
}
