package handler

import (
	"bytes"
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
)

//go:generate go run golang.org/x/tools/cmd/stringer -type=EventType
type EventType int

const (
	NullEvent EventType = iota
	ApiRequest
	MailtoEvent
	SnsEvent
)

type Event struct {
	Type        EventType
	ApiRequest  *events.APIGatewayV2HTTPRequest
	MailtoEvent *events.SimpleEmailEvent
	SnsEvent    *events.SNSEvent
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
		event.ApiRequest = &events.APIGatewayV2HTTPRequest{}
		return json.Unmarshal(data, event.ApiRequest)
	} else if bytes.Contains(data, []byte(`"ses":`)) {
		event.Type = MailtoEvent
		event.MailtoEvent = &events.SimpleEmailEvent{}
		return json.Unmarshal(data, event.MailtoEvent)
	} else if bytes.Contains(data, []byte(`"Sns":`)) {
		event.Type = SnsEvent
		event.SnsEvent = &events.SNSEvent{}
		return json.Unmarshal(data, event.SnsEvent)
	}
	return nil
}
