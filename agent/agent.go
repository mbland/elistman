package agent

import (
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
	Validate(
		ctx context.Context, address string,
	) (failure *email.ValidationFailure, err error)
	Import(ctx context.Context, address string) (err error)
	Remove(ctx context.Context, email string, reason ops.RemoveReason) error
	Restore(ctx context.Context, email string) error
	Send(ctx context.Context, msg *email.Message) (numSent int, err error)
}

type ProdAgent struct {
	SenderAddress    string
	EmailSiteTitle   string
	EmailDomainName  string
	UnsubscribeEmail string
	ApiBaseUrl       string
	NewUid           func() (uuid.UUID, error)
	CurrentTime      func() time.Time
	Db               db.Database
	Validator        email.AddressValidator
	Mailer           email.Mailer
	Suppressor       email.Suppressor
	Log              *log.Logger
}

func (a *ProdAgent) Subscribe(
	ctx context.Context, address string,
) (result ops.OperationResult, err error) {
	var failure *email.ValidationFailure
	var sub *db.Subscriber

	if failure, err = a.Validate(ctx, address); err != nil {
		return
	} else if failure != nil {
		a.Log.Printf("validation failed: %s", failure)
		return
	} else if sub, err = a.Db.Get(ctx, address); err == nil {
		switch sub.Status {
		case db.SubscriberPending:
			result = ops.VerifyLinkSent
		default:
			result = ops.AlreadySubscribed
		}
		return
	} else if !errors.Is(err, db.ErrSubscriberNotFound) {
		return
	}

	sub = &db.Subscriber{Email: address, Status: db.SubscriberPending}
	if err = a.putSubscriber(ctx, sub); err != nil {
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

// timeToLiveDuration defines how long a pending Subscriber can exist.
//
// putSubscriber adds a day to the timestamp for pending subscribers
// so DynamoDB's Time To Live feature can eventually remove them.
const timeToLiveDuration = time.Hour * 24

func (a *ProdAgent) putSubscriber(
	ctx context.Context, sub *db.Subscriber,
) (err error) {
	sub.Timestamp = a.CurrentTime()

	if sub.Status == db.SubscriberPending {
		sub.Timestamp = sub.Timestamp.Add(timeToLiveDuration)
	}
	if sub.Uid, err = a.NewUid(); err != nil {
		return err
	}
	return a.Db.Put(ctx, sub)
}

const verifySubjectPrefix = "Verify your email subscription to "

const verifyTextFormat = `` +
	`Please verify your email subscription to %s by clicking:

- %s

If you did not subscribe, please ignore this email.
`

const verifyHtmlFormat = `<!DOCTYPE html ` +
	`PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" ` +
	`"https://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="https://www.w3.org/1999/xhtml" lang="en-us">
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
<meta http-equiv="X-UA-Compatible" content="IE=edge" />	
<title>Verify your email subscription to %s</title>
</head>
<body>
<p>Please verify your email subscription to %s by clicking:</p>
<ul><li><a href="%s">%s</a></li></ul>
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
	recipient := &email.Recipient{Email: sub.Email, Uid: sub.Uid}
	mt := email.NewMessageTemplate(&email.Message{
		From:     a.SenderAddress,
		Subject:  verifySubjectPrefix + a.EmailSiteTitle,
		TextBody: verifyTextBody(a.EmailSiteTitle, verifyLink),
		HtmlBody: verifyHtmlBody(a.EmailSiteTitle, verifyLink),
	})
	return mt.GenerateMessage(recipient)
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

func (a *ProdAgent) Validate(
	ctx context.Context, address string,
) (failure *email.ValidationFailure, err error) {
	return a.Validator.ValidateAddress(ctx, address)
}

func (a *ProdAgent) Import(ctx context.Context, address string) (err error) {
	var failure *email.ValidationFailure
	var sub *db.Subscriber

	if failure, err = a.Validate(ctx, address); err != nil {
		return
	} else if failure != nil {
		return errors.New(failure.Reason)
	} else if sub, err = a.Db.Get(ctx, address); err == nil {
		if sub.Status == db.SubscriberVerified {
			return errors.New("already a verified subscriber")
		}
	} else if !errors.Is(err, db.ErrSubscriberNotFound) {
		return
	}
	sub = &db.Subscriber{Email: address, Status: db.SubscriberVerified}
	err = a.putSubscriber(ctx, sub)
	return
}

func (a *ProdAgent) Remove(
	ctx context.Context, address string, reason ops.RemoveReason,
) (err error) {
	if err = a.Db.Delete(ctx, address); err == nil {
		err = a.Suppressor.Suppress(ctx, address, reason)
	}
	return
}

func (a *ProdAgent) Restore(ctx context.Context, address string) (err error) {
	// Since the SnsHandler is calling this to restore a previous subscriber,
	// presume they're already verified.
	sub := &db.Subscriber{Email: address, Status: db.SubscriberVerified}
	if err = a.putSubscriber(ctx, sub); err == nil {
		err = a.Suppressor.Unsuppress(ctx, address)
	}
	return
}

func (a *ProdAgent) Send(
	ctx context.Context, msg *email.Message,
) (numSent int, err error) {
	if err = msg.Validate(email.CheckDomain(a.EmailDomainName)); err != nil {
		return
	} else if err = a.Mailer.BulkCapacityAvailable(ctx); err != nil {
		err = fmt.Errorf("couldn't send to subscribers: %w", err)
		return
	}

	mt := email.NewMessageTemplate(msg)
	var sendErr error

	sender := db.SubscriberFunc(func(sub *db.Subscriber) bool {
		recipient := &email.Recipient{Email: sub.Email, Uid: sub.Uid}
		recipient.SetUnsubscribeInfo(a.UnsubscribeEmail, a.ApiBaseUrl)

		m := mt.GenerateMessage(recipient)
		var msgId string

		if msgId, sendErr = a.Mailer.Send(ctx, sub.Email, m); sendErr != nil {
			return false
		}
		a.Log.Printf("sent \"%s\" id: %s to: %s", msg.Subject, msgId, sub.Email)
		numSent++
		return true
	})

	err = a.Db.ProcessSubscribers(ctx, db.SubscriberVerified, sender)
	if err = errors.Join(err, sendErr); err != nil {
		const errFmt = "error sending \"%s\" to list: %w"
		err = fmt.Errorf(errFmt, msg.Subject, err)
	}
	return
}
