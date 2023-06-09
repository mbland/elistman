//go:build small_tests || all_tests

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/google/uuid"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/events"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type testAgent struct {
	Email             string
	Uid               uuid.UUID
	OpResult          ops.OperationResult
	NumSent           int
	ImportedAddresses []string
	ImportResponse    func(address string) error
	SendResponse      func(msg *email.Message, addrs []string) (int, error)
	Error             error
	Calls             []testAgentCalls
}

type testAgentCalls struct {
	Method string
	Email  string
	Uid    uuid.UUID
	Msg    *email.Message
	Reason ops.RemoveReason
	Addrs  []string
}

func (a *testAgent) Subscribe(
	ctx context.Context, email string,
) (ops.OperationResult, error) {
	a.Calls = append(a.Calls, testAgentCalls{Method: "Subscribe", Email: email})
	a.Email = email
	return a.OpResult, a.Error
}

func (a *testAgent) Verify(
	ctx context.Context, email string, uid uuid.UUID,
) (ops.OperationResult, error) {
	a.Calls = append(a.Calls, testAgentCalls{
		Method: "Verify", Email: email, Uid: uid,
	})
	a.Email = email
	a.Uid = uid
	return a.OpResult, a.Error
}

func (a *testAgent) Unsubscribe(
	ctx context.Context, email string, uid uuid.UUID,
) (ops.OperationResult, error) {
	a.Calls = append(a.Calls, testAgentCalls{
		Method: "Unsubscribe", Email: email, Uid: uid,
	})
	a.Email = email
	a.Uid = uid
	return a.OpResult, a.Error
}

func (a *testAgent) Validate(
	_ context.Context, address string,
) (*email.ValidationFailure, error) {
	// This method isn't used directly by any handlers, but is part of the
	// public API used by the command line interface.
	return nil, nil
}

func (a *testAgent) Import(_ context.Context, address string) (err error) {
	a.ImportedAddresses = append(a.ImportedAddresses, address)
	return a.ImportResponse(address)
}

func (a *testAgent) Remove(
	ctx context.Context, email string, reason ops.RemoveReason,
) error {
	agentCall := testAgentCalls{Method: "Remove", Email: email, Reason: reason}
	a.Calls = append(a.Calls, agentCall)
	a.Email = email
	return a.Error
}

func (a *testAgent) Restore(ctx context.Context, email string) error {
	a.Calls = append(a.Calls, testAgentCalls{Method: "Restore", Email: email})
	a.Email = email
	return a.Error
}

func (a *testAgent) Send(
	ctx context.Context, msg *email.Message, addrs []string,
) (numSent int, err error) {
	call := testAgentCalls{Method: "Send", Msg: msg, Addrs: addrs}
	a.Calls = append(a.Calls, call)
	return a.SendResponse(msg, addrs)
}

const testEmailDomain = "mike-bland.com"
const testSiteTitle = "Mike Bland's blog"
const testUnsubscribeUser = "unsubscribe"
const testUnsubscribeAddress = testUnsubscribeUser + "@" + testEmailDomain
const testValidUidStr = "00000000-1111-2222-3333-444444444444"

var testValidUid uuid.UUID = uuid.MustParse(testValidUidStr)

var testRedirects = RedirectPaths{
	Invalid:           "invalid",
	AlreadySubscribed: "already-subscribed",
	VerifyLinkSent:    "verify-link-sent",
	Subscribed:        "subscribed",
	NotSubscribed:     "not-subscribed",
	Unsubscribed:      "unsubscribed",
}

type testBouncer struct {
	EmailDomain     string
	MessageId       string
	Recipients      []string
	Timestamp       time.Time
	ReturnMessageId string
	Error           error
}

func (b *testBouncer) Bounce(
	ctx context.Context,
	emailDomain,
	messageId string,
	recipients []string,
	timestamp time.Time,
) (string, error) {
	b.EmailDomain = emailDomain
	b.MessageId = messageId
	b.Recipients = recipients
	b.Timestamp = timestamp
	if b.Error != nil {
		return "", b.Error
	}
	return b.ReturnMessageId, nil
}

type handlerFixture struct {
	agent   *testAgent
	logs    *testutils.Logs
	bouncer *testBouncer
	handler *Handler
	ctx     context.Context
	event   *Event
}

func newHandlerFixture() *handlerFixture {
	logs, logger := testutils.NewLogs()
	agent := &testAgent{}
	bouncer := &testBouncer{}
	ctx := context.Background()
	handler, err := NewHandler(
		testEmailDomain,
		testSiteTitle,
		agent,
		testRedirects,
		ResponseTemplate,
		testUnsubscribeUser,
		bouncer,
		logger,
	)

	if err != nil {
		panic(err.Error())
	}
	return &handlerFixture{agent, logs, bouncer, handler, ctx, &Event{}}
}

func apiGatewayRequest(method, path string) *awsevents.APIGatewayProxyRequest {
	return &awsevents.APIGatewayProxyRequest{
		HTTPMethod: method,
		RequestContext: awsevents.APIGatewayProxyRequestContext{
			RequestID:    "deadbeef",
			ResourcePath: path,
			Protocol:     "HTTP/2",
			Identity: awsevents.APIGatewayRequestIdentity{
				SourceIP: "192.168.0.1",
			},
		},
	}
}

func apiGatewayResponse(status int) *awsevents.APIGatewayProxyResponse {
	return &awsevents.APIGatewayProxyResponse{
		StatusCode: status, Headers: map[string]string{},
	}
}

func testTimestamp() time.Time {
	timestamp, err := time.Parse(time.DateOnly, "1970-09-18")

	if err != nil {
		panic("failed to parse test timestamp: " + err.Error())
	}
	return timestamp
}

// This example matches the fields constructed by newMailtoHandlerFixture().
func simpleEmailService() *awsevents.SimpleEmailService {
	timestamp := testTimestamp()

	return &awsevents.SimpleEmailService{
		Mail: awsevents.SimpleEmailMessage{
			MessageID: "deadbeef",
			CommonHeaders: awsevents.SimpleEmailCommonHeaders{
				From:    []string{"mbland@acm.org"},
				To:      []string{testUnsubscribeAddress},
				Subject: "mbland@acm.org " + testValidUidStr,
			},
		},
		// Set all verdicts and DMARCPolicy to lowercase here to make sure that
		// TestNewMailtoEvent validates that newMailtoHandler() uppercases them
		// all.
		Receipt: awsevents.SimpleEmailReceipt{
			Recipients:   []string{testUnsubscribeAddress},
			Timestamp:    timestamp,
			SPFVerdict:   awsevents.SimpleEmailVerdict{Status: "pass"},
			DKIMVerdict:  awsevents.SimpleEmailVerdict{Status: "pass"},
			SpamVerdict:  awsevents.SimpleEmailVerdict{Status: "pass"},
			VirusVerdict: awsevents.SimpleEmailVerdict{Status: "pass"},
			DMARCVerdict: awsevents.SimpleEmailVerdict{Status: "pass"},
			DMARCPolicy:  "reject",
		},
	}
}

func simpleEmailEvent() *awsevents.SimpleEmailEvent {
	event := &awsevents.SimpleEmailEvent{
		Records: []awsevents.SimpleEmailRecord{{SES: *simpleEmailService()}},
	}
	return event
}

func simpleNotificationServiceEvent() *awsevents.SNSEvent {
	encodedMsg, err := json.Marshal(sesEventRecord())

	if err != nil {
		panic("failed to json.Marshal test SesEventRecord: " + err.Error())
	}
	return &awsevents.SNSEvent{
		Records: []awsevents.SNSEventRecord{
			{
				EventVersion:         "1.0",
				EventSubscriptionArn: "aws:sns:us-east-1:0123456789:foo/bar",
				EventSource:          "aws:sns",
				SNS: awsevents.SNSEntity{
					Timestamp: testTimestamp(),
					MessageID: "deadbeef",
					Type:      "Notification",
					Message:   string(encodedMsg),
				},
			},
		},
	}
}

func sesEventRecord() *events.SesEventRecord {
	return &events.SesEventRecord{
		EventType: "Send",
		Send:      &events.SesSendEvent{},
		Mail: events.SesEventMessage{
			SimpleEmailMessage: awsevents.SimpleEmailMessage{
				MessageID: "deadbeef",
				CommonHeaders: awsevents.SimpleEmailCommonHeaders{
					From:    []string{"mbland@acm.org"},
					To:      []string{"foo@bar.com"},
					Subject: "This is an email sent to the list",
				},
			},
			Tags: map[string][]string{
				"foo": {"bar"},
			},
		},
	}
}

func TestNewHandler(t *testing.T) {
	newHandler := func(responseTemplate string) (*Handler, error) {
		return NewHandler(
			testEmailDomain,
			testSiteTitle,
			&testAgent{},
			testRedirects,
			responseTemplate,
			testUnsubscribeUser,
			&testBouncer{},
			&log.Logger{},
		)
	}

	t.Run("Succeeds", func(t *testing.T) {
		handler, err := newHandler(ResponseTemplate)

		assert.NilError(t, err)
		assert.Equal(t, testSiteTitle, handler.api.SiteTitle)
		assert.Equal(t, testUnsubscribeAddress, handler.mailto.UnsubscribeAddr)
		assert.Assert(t, handler.sns != nil)
	})

	t.Run("ReturnsErrorIfBadResponseTemplate", func(t *testing.T) {
		handler, err := newHandler("{{.Bogus}}")

		assert.Assert(t, is.Nil(handler))
		assert.Assert(t, err != nil)
	})
}

func TestHandleEvent(t *testing.T) {
	t.Run("ReturnsSuccessfulApiResponse", func(t *testing.T) {
		f := newHandlerFixture()
		f.event.Type = ApiRequest
		f.agent.OpResult = ops.VerifyLinkSent

		req := apiGatewayRequest(http.MethodPost, ops.ApiPrefixSubscribe)
		req.Headers = map[string]string{
			"content-type": "application/x-www-form-urlencoded",
		}
		req.Body = "email=mbland%40acm.org"
		f.event.ApiRequest = req

		response, err := f.handler.HandleEvent(f.ctx, f.event)

		assert.NilError(t, err)
		assert.Equal(t, "mbland@acm.org", f.agent.Email)
		apiResponse, ok := response.(*awsevents.APIGatewayProxyResponse)
		assert.Assert(t, ok)
		assert.Equal(t, http.StatusSeeOther, apiResponse.StatusCode)
		expectedRedirect := f.handler.api.Redirects[ops.VerifyLinkSent]
		assert.Equal(t, expectedRedirect, apiResponse.Headers["location"])
	})

	t.Run("HandlesSuccessfulMailtoEvent", func(t *testing.T) {
		f := newHandlerFixture()
		f.event.Type = MailtoEvent
		f.event.MailtoEvent = simpleEmailEvent()
		f.agent.OpResult = ops.Unsubscribed

		response, err := f.handler.HandleEvent(f.ctx, f.event)

		assert.NilError(t, err)
		expected := &awsevents.SimpleEmailDisposition{
			Disposition: awsevents.SimpleEmailStopRuleSet,
		}
		assert.DeepEqual(t, expected, response)
		assert.Equal(t, "mbland@acm.org", f.agent.Email)
		assert.Equal(t, testValidUid, f.agent.Uid)
		f.logs.AssertContains(t, "success")
	})

	t.Run("HandleSuccessfulSnsEvent", func(t *testing.T) {
		f := newHandlerFixture()
		f.event.Type = SnsEvent
		f.event.SnsEvent = simpleNotificationServiceEvent()

		response, err := f.handler.HandleEvent(f.ctx, f.event)

		assert.NilError(t, err)
		assert.Assert(t, is.Nil(response))
		f.logs.AssertContains(t, "Send")
		f.logs.AssertContains(t, `Subject:"This is an email sent to the list"`)
		f.logs.AssertContains(t, "success")
	})

	t.Run("HandleSuccessfulCommandLineEvent", func(t *testing.T) {
		f := newHandlerFixture()
		f.event.Type = CommandLineEvent
		f.event.CommandLineEvent = &events.CommandLineEvent{
			EListManCommand: events.CommandLineSendEvent,
			Send:            &events.SendEvent{Message: *email.ExampleMessage},
		}
		numSent := 27
		f.agent.SendResponse = func(_ *email.Message, _ []string) (int, error) {
			return numSent, nil
		}

		response, err := f.handler.HandleEvent(f.ctx, f.event)

		assert.NilError(t, err)
		expected := &events.SendResponse{Success: true, NumSent: numSent}
		assert.DeepEqual(t, expected, response)
		f.logs.AssertContains(
			t, "send: subject: \""+email.ExampleMessage.Subject+"\"",
		)
	})

	t.Run("HandleUnknownCommandLineEvent", func(t *testing.T) {
		f := newHandlerFixture()
		f.event.Type = CommandLineEvent
		f.event.CommandLineEvent = &events.CommandLineEvent{
			EListManCommand: events.CommandLineEventType("unknown"),
		}

		response, err := f.handler.HandleEvent(f.ctx, f.event)

		assert.Assert(t, is.Nil(response))
		assert.ErrorContains(t, err, "unknown EListMan command: unknown")
	})

	t.Run("ReturnsErrorOnUnknownEvent", func(t *testing.T) {
		f := newHandlerFixture()
		f.event.Unknown = []byte(`{ "foo": "bar" }`)

		response, err := f.handler.HandleEvent(f.ctx, f.event)

		assert.Equal(t, nil, response)
		const errFmt = "unknown event: %s"
		assert.Error(t, err, fmt.Sprintf(errFmt, f.event.Unknown))
	})

	t.Run("ReturnsErrorOnUnexpectedEvent", func(t *testing.T) {
		f := newHandlerFixture()
		// To simulate an unexpected event, screw up the event.Type of an
		// otherwise empty event.
		f.event.Type = UnknownEvent - 1

		response, err := f.handler.HandleEvent(f.ctx, f.event)

		assert.Equal(t, nil, response)
		const errFmt = "unexpected event type: %s: %+v"
		assert.Error(t, err, fmt.Sprintf(errFmt, f.event.Type, f.event))
	})
}
