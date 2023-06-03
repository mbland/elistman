package handler

import (
	"bytes"
	"encoding/json"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/events"
)

//go:generate go run golang.org/x/tools/cmd/stringer -type=EventType
type EventType int

const (
	UnknownEvent EventType = iota
	ApiRequest
	MailtoEvent
	SnsEvent
	CommandLineEvent
)

type Event struct {
	Type             EventType
	ApiRequest       *awsevents.APIGatewayV2HTTPRequest
	MailtoEvent      *awsevents.SimpleEmailEvent
	SnsEvent         *awsevents.SNSEvent
	CommandLineEvent *events.SendEvent
	Unknown          []byte
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
		event.ApiRequest = &awsevents.APIGatewayV2HTTPRequest{}
		return json.Unmarshal(data, event.ApiRequest)
	} else if bytes.Contains(data, []byte(`"ses":`)) {
		event.Type = MailtoEvent
		event.MailtoEvent = &awsevents.SimpleEmailEvent{}
		return json.Unmarshal(data, event.MailtoEvent)
	} else if bytes.Contains(data, []byte(`"Sns":`)) {
		event.Type = SnsEvent
		event.SnsEvent = &awsevents.SNSEvent{}
		return json.Unmarshal(data, event.SnsEvent)
	} else if bytes.Contains(data, []byte(email.UnsubscribeUrlTemplate)) {
		event.Type = CommandLineEvent
		event.CommandLineEvent = &events.SendEvent{}
		return json.Unmarshal(data, event.CommandLineEvent)
	}
	event.Unknown = data
	return nil
}
