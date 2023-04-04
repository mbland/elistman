package handler

import (
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"gotest.tools/assert"
)

func TestUnknownEvent(t *testing.T) {
	unknownEvent := NullEvent - 1
	assert.Equal(t, "Unknown", unknownEvent.String())
}

func TestUnmarshalNullEventIsNop(t *testing.T) {
	e := Event{}

	err := e.UnmarshalJSON([]byte("null"))

	assert.NilError(t, err)
	assert.Equal(t, "Null", e.Type.String())
	assert.DeepEqual(t, Event{}, e)
}

func TestUnmarshalUnknownEventIsNop(t *testing.T) {
	e := Event{}

	err := e.UnmarshalJSON([]byte(`{ "foo": "bar" }`))

	assert.NilError(t, err)
	assert.Equal(t, "Null", e.Type.String())
	assert.DeepEqual(t, Event{}, e)
}

const apiRequestJson = `{
	"version": "2.0",
	"routeKey": "POST /subscribe",
	"rawPath": "/subscribe"
}`

func TestApiRequest(t *testing.T) {
	e := Event{}

	err := e.UnmarshalJSON([]byte(apiRequestJson))

	assert.NilError(t, err)
	assert.Equal(t, "API Request", e.Type.String())
	assert.DeepEqual(t, e, Event{
		Type: ApiRequest,
		ApiRequest: &events.APIGatewayV2HTTPRequest{
			Version:  "2.0",
			RouteKey: "POST /subscribe",
			RawPath:  "/subscribe",
		},
	})
}

const mailtoEventJson string = `{
	"Records": [
		{
			"eventVersion": "1.0",
			"eventSource": "ses.amazonaws.com",
			"ses": {
				"mail": {
					"commonHeaders": {
						"to": [ "unsubscribe@mike-bland.com" ],
						"subject": "foo@bar.com UID"
					}
				}
			}
		}
	]
}`

func TestMailtoEvent(t *testing.T) {
	e := Event{}

	err := e.UnmarshalJSON([]byte(mailtoEventJson))

	assert.NilError(t, err)
	assert.Equal(t, "Mailto", e.Type.String())
	assert.DeepEqual(t, e, Event{
		Type: MailtoEvent,
		MailtoEvent: &events.SimpleEmailEvent{
			Records: []events.SimpleEmailRecord{
				{
					EventVersion: "1.0",
					EventSource:  "ses.amazonaws.com",
					SES: events.SimpleEmailService{
						Mail: events.SimpleEmailMessage{
							CommonHeaders: events.SimpleEmailCommonHeaders{
								To:      []string{"unsubscribe@mike-bland.com"},
								Subject: "foo@bar.com UID",
							},
						},
					},
				},
			},
		},
	})
}
