package types

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

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

const SubscriberTimestampFormat = time.RFC1123Z

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
	sb.WriteString(sub.Timestamp.Format(SubscriberTimestampFormat))
	return sb.String()
}
