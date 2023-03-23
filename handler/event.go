package handler

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

type EventType int

const (
	UnexpectedEvent EventType = iota - 1
	NullEvent
	ApiRequest
	MailtoEvent
)

func (event EventType) String() string {
	switch event {
	case UnexpectedEvent:
		return "Unexpected event"
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
	} else if bytes.Contains(data, []byte(`"rawPath":`)) {
		event.Type = ApiRequest
		return json.Unmarshal(data, &event.ApiRequest)
	} else if bytes.Contains(data, []byte(`"ses":`)) {
		event.Type = MailtoEvent
		return json.Unmarshal(data, &event.MailtoEvent)
	}
	event.Type = UnexpectedEvent
	return fmt.Errorf("failed to parse unexpected event: %s", string(data[:]))
}
