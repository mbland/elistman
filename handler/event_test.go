package handler

import (
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"gotest.tools/assert"
)

func TestUnknownEvent(t *testing.T) {
	unknownEvent := UnexpectedEvent - 1
	assert.Equal(t, "Unknown event", unknownEvent.String())
}

func TestUnmarshalNullEvent(t *testing.T) {
	e := Event{}

	err := e.UnmarshalJSON([]byte("null"))

	assert.NilError(t, err)
	assert.Equal(t, "Null event", e.Type.String())
	assert.DeepEqual(t, Event{}, e)
}

func TestUnmarshalInvalidEvent(t *testing.T) {
	e := Event{}

	err := e.UnmarshalJSON([]byte(`{ "foo": "bar" }`))

	assert.Error(t, err, `failed to parse unexpected event: { "foo": "bar" }`)
	assert.Equal(t, "Unexpected event", e.Type.String())
	assert.DeepEqual(t, Event{Type: UnexpectedEvent}, e)
}

const apiRequest = `{
	"version": "2.0",
	"routeKey": "POST /subscribe",
	"rawPath": "/subscribe"
}`

func TestApiRequest(t *testing.T) {
	e := Event{}

	err := e.UnmarshalJSON([]byte(apiRequest))

	assert.NilError(t, err)
	assert.Equal(t, "API Request event", e.Type.String())
	assert.DeepEqual(t, e, Event{
		Type: ApiRequest,
		ApiRequest: events.APIGatewayV2HTTPRequest{
			Version:  "2.0",
			RouteKey: "POST /subscribe",
			RawPath:  "/subscribe",
		},
	})
}

const mailtoEvent string = `{
	"Records": [
		{
			"eventVersion": "1.0",
			"eventSource": "ses.amazonaws.com",
			"ses": {
			}
		}
	]
}`

func TestMailtoEvent(t *testing.T) {
	e := Event{}

	err := e.UnmarshalJSON([]byte(mailtoEvent))

	assert.NilError(t, err)
	assert.Equal(t, "Mailto event", e.Type.String())
	assert.DeepEqual(t, e, Event{
		Type: MailtoEvent,
		MailtoEvent: events.SimpleEmailEvent{
			Records: []events.SimpleEmailRecord{
				{
					EventVersion: "1.0",
					EventSource:  "ses.amazonaws.com",
					SES:          events.SimpleEmailService{},
				},
			},
		},
	})
}
