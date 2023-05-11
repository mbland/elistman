package agent

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/ops"
)

type DecoyAgent struct {
	SenderAddress      string
	UnsubscribeEmail   string
	UnsubscribeBaseUrl string
	NewUid             func() (uuid.UUID, error)
	CurrentTime        func() time.Time
	Db                 db.Database
	Validator          email.AddressValidator
	Mailer             email.Mailer
	Logger             *log.Logger
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

func (a *DecoyAgent) Restore(ctx context.Context, email string) error {
	return nil
}
