package db

import (
	"time"

	"github.com/google/uuid"
)

type Database interface {
	Get(email string) (*Subscriber, error)
	Put(subscriber *Subscriber) error
	Delete(email string) error
}

type Subscriber struct {
	Email     string
	Uid       uuid.UUID
	Verified  bool
	Timestamp time.Time
}

func NewSubscriber(email string) *Subscriber {
	return &Subscriber{
		Email:     email,
		Uid:       uuid.New(),
		Verified:  false,
		Timestamp: time.Now().Truncate(time.Second),
	}
}
