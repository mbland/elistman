package db

import (
	"context"

	"github.com/mbland/elistman/types"
)

type Database interface {
	Get(ctx context.Context, email string) (*types.Subscriber, error)
	Put(ctx context.Context, subscriber *types.Subscriber) error
	Delete(ctx context.Context, email string) error
	ProcessSubscribersInState(
		context.Context, types.SubscriberStatus, SubscriberProcessor,
	) error
}

// ErrSubscriberNotFound indicates that an email address isn't subscribed.
//
// Database.Get returns this error when the underlying database request
// succeeded, but there was no such Subscriber.
const ErrSubscriberNotFound = types.SentinelError("is not a subscriber")

// A SubscriberProcessor performs an operation on a Subscriber.
//
// Process should return true if processing should continue with the next
// Subscriber, or false if processing should halt.
type SubscriberProcessor interface {
	Process(*types.Subscriber) bool
}

// SubscriberFunc is an adapter to allow processing of Subscriber
// objects using plain functions.
//
// Inspired by: https://pkg.go.dev/net/http#HandlerFunc
type SubscriberFunc func(sub *types.Subscriber) bool

// SubscriberFunc calls and returns f(sub).
func (f SubscriberFunc) Process(sub *types.Subscriber) bool {
	return f(sub)
}
