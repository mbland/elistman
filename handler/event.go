package handler

import (
	"bytes"
	"encoding/json"

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
		return "Null"
	case ApiRequest:
		return "API Request"
	case MailtoEvent:
		return "Mailto"
	}
	return "Unknown"
}

type Event struct {
	Type        EventType
	ApiRequest  events.APIGatewayV2HTTPRequest
	MailtoEvent events.SimpleEmailEvent
}

// Inspired by:
// - https://www.synvert-tcm.com/blog/handling-multiple-aws-lambda-event-types-with-go/
// See also:
// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-event.html
func (event *Event) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	} else if bytes.Contains(data, []byte(`"rawPath":`)) {
		event.Type = ApiRequest
		return json.Unmarshal(data, &event.ApiRequest)
	} else if bytes.Contains(data, []byte(`"commonHeaders":`)) {
		event.Type = MailtoEvent
		return json.Unmarshal(data, &event.MailtoEvent)
	}
	return nil
}
