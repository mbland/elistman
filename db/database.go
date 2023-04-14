package db

import (
	"fmt"
	"strings"
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
	Status    SubscriberStatus
	Timestamp time.Time
}

func NewSubscriber(email string) *Subscriber {
	return &Subscriber{
		Email:     email,
		Uid:       uuid.New(),
		Status:    Unverified,
		Timestamp: time.Now().Truncate(time.Second),
	}
}

//go:generate go run golang.org/x/tools/cmd/stringer -type=SubscriberStatus
type SubscriberStatus int

const (
	Unverified SubscriberStatus = iota
	Verified
)

// This might be worth trying to contribute upstream to the stringer project one
// day.
func ParseSubscriberStatus(status string) (SubscriberStatus, error) {
	nameIndex := strings.Index(_SubscriberStatus_name, status)
	for i := range _SubscriberStatus_index {
		if i == nameIndex {
			return SubscriberStatus(i), nil
		}
	}
	return SubscriberStatus(nameIndex),
		fmt.Errorf("unknown SubscriberStatus: %s", status)
}
