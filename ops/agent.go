package ops

import (
	"github.com/google/uuid"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
)

type SubscriptionAgent interface {
	Subscribe(email string) (bool, error)
	Verify(email string, uid uuid.UUID) (bool, error)
	Unsubscribe(email string, uid uuid.UUID) (bool, error)
}

type ProdAgent struct {
	Db        db.Database
	Validator email.AddressValidator
	Mailer    email.Mailer
}

func (h ProdAgent) Subscribe(email string) (bool, error) {
	return true, nil
}

func (h ProdAgent) Verify(email string, uid uuid.UUID) (bool, error) {
	return true, nil
}

func (h ProdAgent) Unsubscribe(email string, uid uuid.UUID) (bool, error) {
	return true, nil
}
