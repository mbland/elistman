package agent

import (
	"context"
	"errors"
	"log"
	"time"

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
	NewUid             func() (uuid.UUID, error)
	CurrentTime        func() time.Time
	Db                 db.Database
	Validator          email.AddressValidator
	Mailer             email.Mailer
	Log                *log.Logger
}

func (a *ProdAgent) Subscribe(
	ctx context.Context, address string,
) (result ops.OperationResult, err error) {
	var failure *email.ValidationFailure
	var sub *db.Subscriber

	if failure, err = a.Validator.ValidateAddress(ctx, address); err != nil {
		return
	} else if failure != nil {
		a.Log.Printf("%s failed validation: %s", address, failure)
		return
	} else if sub, err = a.getOrCreateSubscriber(ctx, address); err != nil {
		return
	} else if sub.Status == db.SubscriberVerified {
		result = ops.AlreadySubscribed
		return
	}
	result = ops.VerifyLinkSent
	return
}

func (a *ProdAgent) getOrCreateSubscriber(
	ctx context.Context, address string,
) (sub *db.Subscriber, err error) {
	sub, err = a.Db.Get(ctx, address)

	if errors.Is(err, db.ErrSubscriberNotFound) {
		sub = &db.Subscriber{Email: address, Status: db.SubscriberPending}
	} else if err != nil || sub.Status == db.SubscriberVerified {
		return
	}

	sub.Timestamp = a.CurrentTime()
	if sub.Uid, err = a.NewUid(); err != nil {
		sub = nil
	} else if err = a.Db.Put(ctx, sub); err != nil {
		sub = nil
	}
	return
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
