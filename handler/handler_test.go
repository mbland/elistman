package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/uuid"
	"github.com/mbland/elistman/ops"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type testAgent struct {
	Email       string
	Uid         uuid.UUID
	ReturnValue ops.OperationResult
	Error       error
}

func (a *testAgent) Subscribe(email string) (ops.OperationResult, error) {
	a.Email = email
	return a.ReturnValue, a.Error
}

func (a *testAgent) Verify(
	email string, uid uuid.UUID,
) (ops.OperationResult, error) {
	a.Email = email
	a.Uid = uid
	return a.ReturnValue, a.Error
}

func (a *testAgent) Unsubscribe(
	email string, uid uuid.UUID,
) (ops.OperationResult, error) {
	a.Email = email
	a.Uid = uid
	return a.ReturnValue, a.Error
}

const testEmailDomain = "mike-bland.com"
const testSiteTitle = "Mike Bland's blog"
const testUnsubscribeAddress = "unsubscribe@" + testEmailDomain
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
	Recipients      []string
	Timestamp       time.Time
	ReturnMessageId string
	Error           error
}

func (b *testBouncer) Bounce(
	emailDomain string, recipients []string, timestamp time.Time,
) (string, error) {
	b.EmailDomain = emailDomain
	b.Recipients = recipients
	b.Timestamp = timestamp
	if b.Error != nil {
		return "", b.Error
	}
	return b.ReturnMessageId, nil
}

type handlerFixture struct {
	agent   *testAgent
	logs    *strings.Builder
	bouncer *testBouncer
	handler *Handler
	event   *Event
}

func newHandlerFixture() *handlerFixture {
	logs, logger := testLogger()
	agent := &testAgent{}
	bouncer := &testBouncer{}
	handler, err := NewHandler(
		testEmailDomain,
		testSiteTitle,
		agent,
		testRedirects,
		ResponseTemplate,
		bouncer,
		logger,
	)

	if err != nil {
		panic(err.Error())
	}
	return &handlerFixture{agent, logs, bouncer, handler, &Event{}}
}

func testLogger() (*strings.Builder, *log.Logger) {
	builder := &strings.Builder{}
	logger := log.New(builder, "test logger: ", 0)
	return builder, logger
}

func apiGatewayRequest(method, path string) *events.APIGatewayV2HTTPRequest {
	return &events.APIGatewayV2HTTPRequest{
		RawPath: path,
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			RequestID: "deadbeef",
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				SourceIP: "192.168.0.1",
				Method:   method,
				Path:     path,
				Protocol: "HTTP/2",
			},
		},
	}
}

func apiGatewayResponse(status int) *events.APIGatewayV2HTTPResponse {
	return &events.APIGatewayV2HTTPResponse{
		StatusCode: status, Headers: map[string]string{},
	}
}

// This example matches the fields constructed by newMailtoHandlerFixture().
func simpleEmailService() *events.SimpleEmailService {
	timestamp, err := time.Parse(time.DateOnly, "1970-09-18")

	if err != nil {
		panic("failed to parse simpleEmailService timestamp: " + err.Error())
	}

	return &events.SimpleEmailService{
		Mail: events.SimpleEmailMessage{
			MessageID: "deadbeef",
			CommonHeaders: events.SimpleEmailCommonHeaders{
				From:    []string{"mbland@acm.org"},
				To:      []string{testUnsubscribeAddress},
				Subject: "mbland@acm.org " + testValidUidStr,
			},
		},
		// Set all verdicts and DMARCPolicy to lowercase here to make sure that
		// TestNewMailtoEvent validates that newMailtoHandler() uppercases them
		// all.
		Receipt: events.SimpleEmailReceipt{
			Recipients:   []string{testUnsubscribeAddress},
			Timestamp:    timestamp,
			SPFVerdict:   events.SimpleEmailVerdict{Status: "pass"},
			DKIMVerdict:  events.SimpleEmailVerdict{Status: "pass"},
			SpamVerdict:  events.SimpleEmailVerdict{Status: "pass"},
			VirusVerdict: events.SimpleEmailVerdict{Status: "pass"},
			DMARCVerdict: events.SimpleEmailVerdict{Status: "pass"},
			DMARCPolicy:  "reject",
		},
	}
}

func simpleEmailEvent() *events.SimpleEmailEvent {
	event := &events.SimpleEmailEvent{
		Records: []events.SimpleEmailRecord{{SES: *simpleEmailService()}},
	}
	return event
}

func TestNewHandler(t *testing.T) {
	newHandler := func(responseTemplate string) (*Handler, error) {
		return NewHandler(
			testEmailDomain,
			testSiteTitle,
			&testAgent{},
			testRedirects,
			responseTemplate,
			&testBouncer{},
			&log.Logger{},
		)
	}

	t.Run("Succeeds", func(t *testing.T) {
		handler, err := newHandler(ResponseTemplate)

		assert.NilError(t, err)
		assert.Equal(t, testSiteTitle, handler.api.SiteTitle)
		assert.Equal(t, testUnsubscribeAddress, handler.mailto.unsubscribeAddr)
	})

	t.Run("ReturnsErrorIfBadResponseTemplate", func(t *testing.T) {
		handler, err := newHandler("{{.Bogus}}")

		assert.Assert(t, is.Nil(handler))
		assert.Assert(t, err != nil)
	})
}

func TestHandleEvent(t *testing.T) {
	t.Run("ReturnsErrorOnUnexpectedEvent", func(t *testing.T) {
		f := newHandlerFixture()

		response, err := f.handler.HandleEvent(f.event)

		assert.Equal(t, nil, response)
		expected := fmt.Sprintf(
			"unexpected event type: %s: %+v", NullEvent, f.event,
		)
		assert.Error(t, err, expected)
	})

	t.Run("ReturnsSuccessfulApiResponse", func(t *testing.T) {
		f := newHandlerFixture()
		f.event.Type = ApiRequest
		f.agent.ReturnValue = ops.VerifyLinkSent

		req := apiGatewayRequest(http.MethodPost, SubscribePrefix)
		req.Headers = map[string]string{
			"content-type": "application/x-www-form-urlencoded",
		}
		req.Body = "email=mbland%40acm.org"
		f.event.ApiRequest = req

		response, err := f.handler.HandleEvent(f.event)

		assert.NilError(t, err)
		assert.Equal(t, "mbland@acm.org", f.agent.Email)
		apiResponse, ok := response.(*events.APIGatewayV2HTTPResponse)
		assert.Assert(t, ok)
		assert.Equal(t, http.StatusSeeOther, apiResponse.StatusCode)
		expectedRedirect := f.handler.api.Redirects[ops.VerifyLinkSent]
		assert.Equal(t, expectedRedirect, apiResponse.Headers["location"])
	})

	t.Run("HandlesSuccessfulMailtoEvent", func(t *testing.T) {
		f := newHandlerFixture()
		f.event.Type = MailtoEvent
		f.event.MailtoEvent = simpleEmailEvent()
		f.agent.ReturnValue = ops.Unsubscribed

		response, err := f.handler.HandleEvent(f.event)
		assert.NilError(t, err)
		assert.Assert(t, is.Nil(response))
		assert.Equal(t, "mbland@acm.org", f.agent.Email)
		assert.Equal(t, testValidUid, f.agent.Uid)
		assert.Assert(
			t, is.Contains(f.logs.String(), "success: mbland@acm.org"),
		)
	})
}
