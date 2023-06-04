//go:build small_tests || all_tests

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/events"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type snsHandlerFixture struct {
	agent   *testAgent
	logs    *testutils.Logs
	handler *snsHandler
	ctx     context.Context
}

func newSnsHandlerFixture() *snsHandlerFixture {
	logs, logger := testutils.NewLogs()
	agent := &testAgent{}
	ctx := context.Background()

	return &snsHandlerFixture{agent, logs, &snsHandler{agent, logger}, ctx}
}

// This and other test messages adapted from:
// https://docs.aws.amazon.com/ses/latest/dg/event-publishing-retrieving-sns-examples.html
const testMailJson = `
  "mail": {
    "timestamp": "1970-09-18T12:45:00.000Z",
    "source": "no-reply@mike-bland.com",
    "sourceArn": "arn:aws:ses:us-east-1:123456789012:identity/mike-bland.com",
    "sendingAccountId": "123456789012",
    "messageId": "EXAMPLE7c191be45",
    "destination": [
      "recipient@example.com"
    ],
    "headersTruncated": false,
    "headers": [
      { "name": "From", "value": "no-reply@mike-bland.com" },
      { "name": "To", "value": "recipient@example.com" },
      { "name": "Subject", "value": "Message sent from Amazon SES" },
      { "name": "MIME-Version", "value": "1.0" },
      {
        "name": "Content-Type",
        "value": "multipart/mixed;  boundary=\"----=_Part_0_716996660.1476\""
      },
      {
        "name": "X-SES-MESSAGE-TAGS",
        "value": "myCustomTag1=myCustomTagVal1, myCustomTag2=myCustomTagVal2"
      }
    ],
    "commonHeaders": {
      "from": [ "no-reply@mike-bland.com" ],
      "to": [ "recipient@example.com" ],
      "messageId": "EXAMPLE7c191be45",
      "subject": "Test message"
    },
    "tags": {
      "ses:configuration-set": [ "ConfigSet" ],
      "ses:source-ip": [ "127.0.0.1" ],
      "ses:from-domain": [ "mike-bland.com" ],      
      "ses:caller-identity": [ "ses_user" ],
      "myCustomTag1": [ "myCustomTagValue1" ],
      "myCustomTag2": [ "myCustomTagValue2" ]      
    }
  }
`

const unimplementedEventJson = `
	{
		"eventType": "Open",
		` + testMailJson + `,
		"open": {
			"ipAddress": "127.0.0.1",
			"timestamp": "1970-09-18T12:45:00.000Z",
			"userAgent": "doesn't matter"
		}
	}
`

func testEvent(eventMsg string) (event *events.SesEventRecord) {
	event = &events.SesEventRecord{}
	var err error

	if err = json.Unmarshal([]byte(eventMsg), event); err != nil {
		panic("failed to parse event message: " + err.Error())
	}
	return
}

func assertTypesMatch(t *testing.T, lhs, rhs any) {
	t.Helper()
	assert.Equal(t, reflect.TypeOf(lhs), reflect.TypeOf(rhs))
}

func TestParseSesEvent(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		message := `
			{
				"eventType": "Send",
				` + testMailJson + `,
  				"send": {}
			}`

		event, handler, err := parseSesEvent(message)

		assert.NilError(t, err)
		assert.Equal(t, "Send", event.EventType)
		assert.Equal(t, "Send", handler.Type)

		expected := &baseSesEventHandler{
			MessageId: "EXAMPLE7c191be45",
			To:        []string{"recipient@example.com"},
			From:      []string{"no-reply@mike-bland.com"},
			Subject:   "Test message",
			Details:   message,
		}

		assert.Equal(t, expected.MessageId, handler.MessageId)
		assert.DeepEqual(t, expected.To, handler.To)
		assert.DeepEqual(t, expected.From, handler.From)
		assert.Equal(t, expected.Subject, handler.Subject)
		assert.Equal(t, expected.Details, handler.Details)
	})

	t.Run("FailsOnParseError", func(t *testing.T) {
		event, handler, err := parseSesEvent("")

		assert.Assert(t, is.Nil(event))
		assert.Assert(t, is.Nil(handler))
		assert.ErrorContains(t, err, "unexpected end of JSON input")
	})
}

func TestBaseSesEventHandlerHandleEvent(t *testing.T) {
	t.Run("LogsErrorForUnimplementedEventType", func(t *testing.T) {
		f := newSesEventHandlerFixture()
		event := testEvent(unimplementedEventJson)

		f.handler.HandleEvent(f.ctx, event)

		f.logs.AssertContains(t, "unimplemented event type: Open")
	})

	t.Run("LogsSuccessForSend", func(t *testing.T) {
		f := newSesEventHandlerFixture()
		event := testEvent(`
			{
				"eventType": "Send",
				` + testMailJson + `,
  				"send": {}
			}`,
		)

		f.handler.HandleEvent(f.ctx, event)

		f.logs.AssertContains(t, "Send [Id:")
		f.logs.AssertContains(t, ": success: ")
	})

	t.Run("LogsSuccessForDelivery", func(t *testing.T) {
		f := newSesEventHandlerFixture()
		event := testEvent(`
			{
				"eventType": "Delivery",
				` + testMailJson + `,
				"delivery": {
					"timestamp": "1970-09-18T12:45:00.000Z",
					"processingTimeMillis": 27,
					"recipients": [ "recipient@example.com" ],
					"smtpResponse": "250 2.6.0 Message received",
					"reportingMTA": "mta.example.com"
				}
			}`,
		)
		f.handler.Type = "Delivery"

		f.handler.HandleEvent(f.ctx, event)

		f.logs.AssertContains(t, "Delivery [Id:")
		f.logs.AssertContains(t, ": success: ")
	})
}

func TestRecipientUpdater(t *testing.T) {
	t.Run("ReturnsSuccessfulOutcome", func(t *testing.T) {
		updater := &recipientUpdater{
			func(context.Context, string) error { return nil },
			"updated",
			"error updating",
		}

		result := updater.updateRecipient(
			context.Background(), "mbland@acm.org", "testing",
		)

		assert.Equal(t, "updated mbland@acm.org due to: testing", result)
	})

	t.Run("ReturnsErrorOutcome", func(t *testing.T) {
		updater := &recipientUpdater{
			func(context.Context, string) error { return errors.New("d'oh!") },
			"updated",
			"error updating",
		}

		result := updater.updateRecipient(
			context.Background(), "mbland@acm.org", "testing",
		)

		expected := "error updating mbland@acm.org due to: testing: d'oh!"
		assert.Equal(t, expected, result)
	})
}

type sesEventHandlerFixture struct {
	handler *baseSesEventHandler
	agent   *testAgent
	logs    *testutils.Logs
	ctx     context.Context
}

func newSesEventHandlerFixture() *sesEventHandlerFixture {
	logs, logger := testutils.NewLogs()
	agent := &testAgent{}
	handler := *testBaseSesEventHandler

	handler.Agent = agent
	handler.Log = logger
	return &sesEventHandlerFixture{&handler, agent, logs, context.Background()}
}

var testBaseSesEventHandler = &baseSesEventHandler{
	Type:      "Send",
	MessageId: "deadbeef",
	From:      []string{"no-reply@mike-bland.com"},
	To:        []string{"mbland@acm.org"},
	Subject:   "Latest blog post",
	// Use a stub, since we're assuming the object has already been parsed.
	Details: `{ "description": "stubbed test message" }`,
}

func assertRecipientUpdated(
	t *testing.T, agent *testAgent, method, email string,
) {
	t.Helper()
	calls := []testAgentCalls{{Method: method, Email: email}}
	assert.DeepEqual(t, calls, agent.Calls)
}

func TestBaseSesEventHandler(t *testing.T) {
	t.Run("logOutcome", func(t *testing.T) {
		f := newSesEventHandlerFixture()

		f.handler.logOutcome("LGTM")

		expected := `Send ` +
			`[Id:"deadbeef" From:"no-reply@mike-bland.com" ` +
			`To:"mbland@acm.org" Subject:"Latest blog post"]: LGTM: ` +
			testBaseSesEventHandler.Details
		f.logs.AssertContains(t, expected)
	})

	t.Run("UpdateRecipients", func(t *testing.T) {
		f := newSesEventHandlerFixture()
		f.handler.To = []string{"mbland@acm.org", "foo@bar.com"}
		updater := &recipientUpdater{
			func(context.Context, string) error { return nil },
			"updated",
			"error updating",
		}

		f.handler.updateRecipients(f.ctx, "testing", updater)

		f.logs.AssertContains(t, "updated mbland@acm.org due to: testing")
		f.logs.AssertContains(t, "updated foo@bar.com due to: testing")
	})

	t.Run("RemoveRecipients", func(t *testing.T) {
		f := newSesEventHandlerFixture()

		f.handler.removeRecipients(f.ctx, "testing")

		f.logs.AssertContains(t, "removed mbland@acm.org due to: testing")
		assertRecipientUpdated(t, f.agent, "Remove", "mbland@acm.org")
	})

	t.Run("RestoreRecipients", func(t *testing.T) {
		f := newSesEventHandlerFixture()

		f.handler.restoreRecipients(f.ctx, "testing")

		f.logs.AssertContains(t, "restored mbland@acm.org due to: testing")
		assertRecipientUpdated(t, f.agent, "Restore", "mbland@acm.org")
	})
}

func TestBounceHandler(t *testing.T) {
	setup := func() (handler *bounceHandler, f *sesEventHandlerFixture) {
		f = newSesEventHandlerFixture()
		handler = &bounceHandler{baseSesEventHandler: *f.handler}
		handler.Type = "Bounce"
		return
	}

	t.Run("DoesNotRemoveRecipientsIfTransient", func(t *testing.T) {
		handler, f := setup()
		handler.BounceType = "Transient"
		handler.BounceSubType = "General"

		handler.HandleEvent(f.ctx)

		f.logs.AssertContains(t, "not removing recipients: Transient/General")
		assert.Assert(t, is.Nil(f.agent.Calls))
	})

	t.Run("RemovesRecipientsIfPermanent", func(t *testing.T) {
		handler, f := setup()
		handler.BounceType = "Permanent"
		handler.BounceSubType = "General"

		handler.HandleEvent(f.ctx)

		f.logs.AssertContains(
			t, "removed mbland@acm.org due to: Permanent/General",
		)
		assertRecipientUpdated(t, f.agent, "Remove", "mbland@acm.org")
	})
}

func TestComplaintHandler(t *testing.T) {
	setup := func() (handler *complaintHandler, f *sesEventHandlerFixture) {
		f = newSesEventHandlerFixture()
		handler = &complaintHandler{baseSesEventHandler: *f.handler}
		handler.Type = "Complaint"
		return
	}

	t.Run("RemovesRecipients", func(t *testing.T) {
		const msgPrefix = "removed mbland@acm.org due to: "

		t.Run("IfSubTypeIsNotEmpty", func(t *testing.T) {
			// Normally this wouldn't contain both complaintSubType and
			// complaintFeedbackType, as a nonempty complaintSubType means the
			// message wasn't even sent. The parser should be able to handle
			// both fields being present at the same time regardless.
			handler, f := setup()
			handler.ComplaintSubType = "OnAccountSuppressionList"

			handler.HandleEvent(f.ctx)

			f.logs.AssertContains(t, msgPrefix+"OnAccountSuppressionList")
			assertRecipientUpdated(t, f.agent, "Remove", "mbland@acm.org")
		})

		t.Run("IfFeedbackIsSpamRelated", func(t *testing.T) {
			handler, f := setup()
			handler.ComplaintFeedbackType = "abuse"

			handler.HandleEvent(f.ctx)

			f.logs.AssertContains(t, msgPrefix+"abuse")
			assertRecipientUpdated(t, f.agent, "Remove", "mbland@acm.org")
		})

		t.Run("IfFeedbackIsUnknown", func(t *testing.T) {
			handler, f := setup()

			handler.HandleEvent(f.ctx)

			f.logs.AssertContains(t, msgPrefix+"unknown")
			assertRecipientUpdated(t, f.agent, "Remove", "mbland@acm.org")
		})
	})

	t.Run("RestoresRecipientsIfFeedbackIsNotSpam", func(t *testing.T) {
		handler, f := setup()
		handler.ComplaintFeedbackType = "not-spam"

		handler.HandleEvent(f.ctx)

		f.logs.AssertContains(t, "restored mbland@acm.org due to: not-spam")
		assertRecipientUpdated(t, f.agent, "Restore", "mbland@acm.org")
	})
}

func TestRejectHandler(t *testing.T) {
	setup := func() (handler *rejectHandler, f *sesEventHandlerFixture) {
		f = newSesEventHandlerFixture()
		handler = &rejectHandler{baseSesEventHandler: *f.handler}
		handler.Type = "Reject"
		return
	}

	t.Run("LogsReason", func(t *testing.T) {
		handler, f := setup()
		handler.Reason = "Bad content"

		handler.HandleEvent(f.ctx)

		f.logs.AssertContains(t, "Bad content")
		assert.Assert(t, is.Nil(f.agent.Calls))
	})
}

func TestHandleSnsEvent(t *testing.T) {
	t.Run("DoesNothingIfNoSnsRecords", func(t *testing.T) {
		f := newSnsHandlerFixture()

		f.handler.HandleEvent(f.ctx, &awsevents.SNSEvent{})

		assert.Equal(t, "", f.logs.Logs())
	})

	t.Run("LogsEventRecordParseError", func(t *testing.T) {
		f := newSnsHandlerFixture()
		event := simpleNotificationServiceEvent()
		event.Records[0].SNS.Message = ""

		f.handler.HandleEvent(f.ctx, event)

		expected := "parsing SES event from SNS failed: " +
			"unexpected end of JSON input: "
		f.logs.AssertContains(t, expected)
	})

	t.Run("LogsErrorForUnimplementedEventType", func(t *testing.T) {
		f := newSnsHandlerFixture()
		event := simpleNotificationServiceEvent()
		event.Records[0].SNS.Message = unimplementedEventJson

		f.handler.HandleEvent(f.ctx, event)

		f.logs.AssertContains(t, "unimplemented event type: Open")
	})

	t.Run("SendEventSucceeds", func(t *testing.T) {
		f := newSnsHandlerFixture()
		event := simpleNotificationServiceEvent()

		f.handler.HandleEvent(f.ctx, event)

		expected := `Send ` +
			`[Id:"deadbeef" From:"mbland@acm.org" To:"foo@bar.com" ` +
			`Subject:"This is an email sent to the list"]: success: ` +
			event.Records[0].SNS.Message
		f.logs.AssertContains(t, expected)
	})
}
