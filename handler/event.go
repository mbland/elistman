package handler

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

type EventType int

const (
	NullEvent EventType = iota
	ApiRequest
	MailtoEvent
)

func (event EventType) String() string {
	switch event {
	case NullEvent:
		return "Null event"
	case ApiRequest:
		return "API Request event"
	case MailtoEvent:
		return "Mailto event"
	}
	return "Unknown event"
}

type Event struct {
	Type        EventType
	ApiRequest  events.APIGatewayV2HTTPRequest
	MailtoEvent events.SimpleEmailEvent
}

// Inspired by:
// https://www.synvert-tcm.com/blog/handling-multiple-aws-lambda-event-types-with-go/
func (event *Event) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	} else if err := json.Unmarshal(data, &event.ApiRequest); err == nil {
		event.Type = ApiRequest
		return nil
	} else if err = json.Unmarshal(data, &event.MailtoEvent); err == nil {
		event.Type = MailtoEvent
		return nil
	}
	return fmt.Errorf("failed to parse incoming event: %s", string(data[:]))
}
