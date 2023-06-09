//go:build small_tests || all_tests

package handler

import (
	"testing"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/events"
	"gotest.tools/assert"
)

func TestEventTypeStrings(t *testing.T) {
	// Just need a little coverage of the stringer-generated code.
	assert.Equal(t, "ApiRequest", ApiRequest.String())

	invalidEvent := UnknownEvent - 1
	assert.Equal(t, "EventType(-1)", invalidEvent.String())
}

func TestUnmarshalNullEventIsNop(t *testing.T) {
	e := Event{}

	err := e.UnmarshalJSON([]byte("null"))

	assert.NilError(t, err)
	assert.DeepEqual(t, Event{}, e)
}

func TestUnmarshalUnknownEvent(t *testing.T) {
	e := Event{}
	unknownPayload := []byte(`{ "foo": "bar" }`)

	err := e.UnmarshalJSON(unknownPayload)

	assert.NilError(t, err)
	assert.DeepEqual(t, Event{Unknown: unknownPayload}, e)
}

const apiRequestJson = `{
	"httpMethod": "POST",
	"requestContext": {
		"resourcePath": "/subscribe"
	}
}`

func TestApiRequest(t *testing.T) {
	e := Event{}

	err := e.UnmarshalJSON([]byte(apiRequestJson))

	assert.NilError(t, err)
	assert.DeepEqual(t, e, Event{
		Type: ApiRequest,
		ApiRequest: &awsevents.APIGatewayProxyRequest{
			HTTPMethod: "POST",
			RequestContext: awsevents.APIGatewayProxyRequestContext{
				ResourcePath: "/subscribe",
			},
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
	assert.DeepEqual(t, e, Event{
		Type: MailtoEvent,
		MailtoEvent: &awsevents.SimpleEmailEvent{
			Records: []awsevents.SimpleEmailRecord{
				{
					EventVersion: "1.0",
					EventSource:  "ses.amazonaws.com",
					SES: awsevents.SimpleEmailService{
						Mail: awsevents.SimpleEmailMessage{
							CommonHeaders: awsevents.SimpleEmailCommonHeaders{
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

const snsEventJson string = `{
	"Records": [
		{
			"EventVersion": "1.0",
			"EventSource":  "aws:sns",
			"Sns": {
				"Message": "stringified JSON object, unmarshalled later"
			}
		}
	]
}`

func TestSnsEvent(t *testing.T) {
	e := Event{}

	err := e.UnmarshalJSON([]byte(snsEventJson))

	assert.NilError(t, err)
	assert.DeepEqual(t, e, Event{
		Type: SnsEvent,
		SnsEvent: &awsevents.SNSEvent{
			Records: []awsevents.SNSEventRecord{
				{
					EventVersion: "1.0",
					EventSource:  "aws:sns",
					SNS: awsevents.SNSEntity{
						Message: "stringified JSON object, unmarshalled later",
					},
				},
			},
		},
	})
}

func TestCommandLineEvent(t *testing.T) {
	const sendEventJson = `{
		"elistmanCommand": "` + events.CommandLineSendEvent + `",
		"send": ` + email.ExampleMessageJson + `
	}`
	e := Event{}

	err := e.UnmarshalJSON([]byte(sendEventJson))

	assert.NilError(t, err)
	assert.DeepEqual(t, e, Event{
		Type: CommandLineEvent,
		CommandLineEvent: &events.CommandLineEvent{
			EListManCommand: events.CommandLineSendEvent,
			Send:            &events.SendEvent{Message: *email.ExampleMessage},
		},
	})
}
