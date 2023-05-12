package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	SenderAddress    string
	EmailSiteTitle   string
	UnsubscribeEmail string
	ApiBaseUrl       string
	NewUid           func() (uuid.UUID, error)
	CurrentTime      func() time.Time
	Db               db.Database
	Validator        email.AddressValidator
	Mailer           email.Mailer
	Log              *log.Logger
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

	msg := a.makeVerificationEmail(sub)
	var msgId string

	if msgId, err = a.Mailer.Send(ctx, address, msg); err == nil {
		a.Log.Printf("sent verification email to %s with ID %s", address, msgId)
		result = ops.VerifyLinkSent
	}
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

const verifySubjectPrefix = "Verify your email subscription to "

const verifyTextFormat = `` +
	`Please verify your email subscription to %s by clicking:

- %s

If you did not subscribe, please ignore this email.
`

const verifyHtmlFormat = `` +
	`<!DOCTYPE html>
<html>
<head><title>Verify your email subscription to %s</title></head>
<body>
<p>Please verify your email subscription to %s by clicking:</p>
<ul><li><a href=\"%s\">%s</a></li></ul>
<p>If you did not subscribe, please ignore this email.</p>
</body>
</html>
`

func verifyTextBody(siteTitle, verifyLink string) string {
	return fmt.Sprintf(verifyTextFormat, siteTitle, verifyLink)
}

func verifyHtmlBody(siteTitle, verifyLink string) string {
	return fmt.Sprintf(
		verifyHtmlFormat, siteTitle, siteTitle, verifyLink, verifyLink,
	)
}

func (a *ProdAgent) makeVerificationEmail(sub *db.Subscriber) []byte {
	verifyLink := ops.VerifyUrl(a.ApiBaseUrl, sub.Email, sub.Uid)
	recipient := &email.Subscriber{Email: sub.Email, Uid: sub.Uid}
	buf := &bytes.Buffer{}

	msg := email.NewMessageTemplate(&email.Message{
		From:     a.SenderAddress,
		Subject:  verifySubjectPrefix + a.EmailSiteTitle,
		TextBody: verifyTextBody(a.EmailSiteTitle, verifyLink),
		HtmlBody: verifyHtmlBody(a.EmailSiteTitle, verifyLink),
	})

	// Don't check the EmitMessage error because bytes.Buffer can essentially
	// never return an error. If it runs out of memory, it panics.
	msg.EmitMessage(buf, recipient)
	return buf.Bytes()
}

func (a *ProdAgent) Verify(
	ctx context.Context, address string, uid uuid.UUID,
) (result ops.OperationResult, err error) {
	var sub *db.Subscriber

	if sub, err = a.getSubscriber(ctx, address, uid); err != nil {
		return
	} else if sub == nil {
		result = ops.NotSubscribed
		return
	} else if sub.Status == db.SubscriberVerified {
		result = ops.AlreadySubscribed
		return
	}

	sub.Status = db.SubscriberVerified
	sub.Timestamp = a.CurrentTime()

	if err = a.Db.Put(ctx, sub); err == nil {
		result = ops.Subscribed
	}
	return
}

func (a *ProdAgent) Unsubscribe(
	ctx context.Context, address string, uid uuid.UUID,
) (result ops.OperationResult, err error) {
	var sub *db.Subscriber

	if sub, err = a.getSubscriber(ctx, address, uid); err != nil {
		return
	} else if sub == nil {
		result = ops.NotSubscribed
	} else if err = a.Db.Delete(ctx, address); err == nil {
		result = ops.Unsubscribed
	}
	return
}

func (a *ProdAgent) getSubscriber(
	ctx context.Context, address string, uid uuid.UUID,
) (sub *db.Subscriber, err error) {
	sub, err = a.Db.Get(ctx, address)

	if errors.Is(err, db.ErrSubscriberNotFound) {
		err = nil
	} else if err != nil {
		return
	} else if sub.Uid != uid {
		sub = nil
	}
	return
}

func (a *ProdAgent) Remove(ctx context.Context, email string) error {
	return nil
}

// This should generate a new UUID as well as remove the user from the
// account-level suppression list if present.
func (a *ProdAgent) Restore(ctx context.Context, email string) error {
	return nil
}
