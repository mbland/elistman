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

// DecoyAgent is a stub implementation of core EListMan business logic.
//
// The earliest deployments of EListMan used DecoyAgent for smoke testing. It
// enabled the smoke tests to ensure that the API Gateway and Lambda function
// were reachable without accessing DynamoDB. It also enabled the smoke test to
// validate that the API request parser worked in an actual deployment. (The
// smoke test uncovered the fact that HTTP/2 lowercases all HTTP headers and
// that API Gateway request bodies are base64 encoded by default.)
//
// EListMan now deploys using ProdAgent, and the smoke tests verify only the
// responses to invalid requests that wouldn't write to DynamoDB anyway.
//
// DecoyAgent's usefulness now is indeed questionable, but it remains for now.
type DecoyAgent struct {
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

func (a *DecoyAgent) Validate(
	_ context.Context, address string,
) (*email.ValidationFailure, error) {
	return nil, nil
}

func (a *DecoyAgent) Import(ctx context.Context, address string) (err error) {
	return nil
}

func (a *DecoyAgent) Remove(
	ctx context.Context, email string, reason ops.RemoveReason) error {
	return nil
}

func (a *DecoyAgent) Restore(ctx context.Context, email string) error {
	return nil
}

func (a *DecoyAgent) Send(
	ctx context.Context, msg *email.Message, addrs []string,
) (numSent int, err error) {
	return 0, nil
}
