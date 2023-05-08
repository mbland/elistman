package db

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// A SubscriberProcessor performs an operation on a Subscriber.
//
// Process should return true if processing should continue with the next
// Subscriber, or false if processing should halt.
type SubscriberProcessor interface {
	Process(*Subscriber) bool
}

// SubscriberFunc is an adapter to allow processing of Subscriber
// objects using plain functions.
//
// Inspired by: https://pkg.go.dev/net/http#HandlerFunc
type SubscriberFunc func(sub *Subscriber) bool

// SubscriberFunc calls and returns f(sub).
func (f SubscriberFunc) Process(sub *Subscriber) bool {
	return f(sub)
}

type Database interface {
	Get(ctx context.Context, email string) (*Subscriber, error)
	Put(ctx context.Context, subscriber *Subscriber) error
	Delete(ctx context.Context, email string) error
	ProcessSubscribersInState(
		context.Context, SubscriberStatus, SubscriberProcessor,
	) error
}

type Subscriber struct {
	Email     string
	Uid       uuid.UUID
	Status    SubscriberStatus
	Timestamp time.Time
}

type SubscriberStatus string

const (
	SubscriberPending  SubscriberStatus = "pending"
	SubscriberVerified SubscriberStatus = "verified"
)

const TimestampFormat = time.RFC1123Z

func NewSubscriber(email string) *Subscriber {
	return &Subscriber{
		Email:     email,
		Uid:       uuid.New(),
		Status:    SubscriberPending,
		Timestamp: time.Now().Truncate(time.Second),
	}
}

func (sub *Subscriber) String() string {
	sb := strings.Builder{}
	sb.WriteString("Email: ")
	sb.WriteString(sub.Email)
	sb.WriteString(", Uid: ")
	sb.WriteString(sub.Uid.String())
	sb.WriteString(", Status: ")
	sb.WriteString(string(sub.Status))
	sb.WriteString(", Timestamp: ")
	sb.WriteString(sub.Timestamp.Format(TimestampFormat))
	return sb.String()
}

const ErrSubscriberNotFound = dbError("is not a subscriber")

type dbError string

func (e dbError) Error() string {
	return string(e)
}
