package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Database interface {
	Get(ctx context.Context, email string) (*Subscriber, error)
	Put(ctx context.Context, subscriber *Subscriber) error
	Delete(ctx context.Context, email string) error
}

type Subscriber struct {
	Email     string
	Uid       uuid.UUID
	Verified  bool
	Timestamp time.Time
}

type SubscriberState string

const (
	SubscriberStatePending  SubscriberState = "pending"
	SubscriberStateVerified SubscriberState = "verified"
)

func NewSubscriber(email string) *Subscriber {
	return &Subscriber{
		Email:     email,
		Uid:       uuid.New(),
		Verified:  false,
		Timestamp: time.Now().Truncate(time.Second),
	}
}

type StartKey interface {
	isDbStartKey()
}
