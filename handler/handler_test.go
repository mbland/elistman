package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"

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

type handlerFixture struct {
	agent   *testAgent
	logs    *strings.Builder
	handler *Handler
	event   *Event
}

func newHandlerFixture() *handlerFixture {
	logs, logger := testLogger()
	agent := &testAgent{}
	handler, err := NewHandler(
		testEmailDomain,
		testSiteTitle,
		agent,
		testRedirects,
		ResponseTemplate,
		logger,
	)

	if err != nil {
		panic(err.Error())
	}
	return &handlerFixture{agent, logs, handler, &Event{}}
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

func simpleEmailService() *events.SimpleEmailService {
	return &events.SimpleEmailService{
		Mail: events.SimpleEmailMessage{
			MessageID: "deadbeef",
			CommonHeaders: events.SimpleEmailCommonHeaders{
				From:    []string{"mbland@acm.org"},
				To:      []string{testUnsubscribeAddress},
				Subject: "mbland@acm.org " + testValidUidStr,
			},
		},
		Receipt: events.SimpleEmailReceipt{
			SPFVerdict:   events.SimpleEmailVerdict{Status: "PASS"},
			DKIMVerdict:  events.SimpleEmailVerdict{Status: "PASS"},
			SpamVerdict:  events.SimpleEmailVerdict{Status: "PASS"},
			VirusVerdict: events.SimpleEmailVerdict{Status: "PASS"},
			DMARCVerdict: events.SimpleEmailVerdict{Status: "PASS"},
			DMARCPolicy:  "REJECT",
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
			&log.Logger{},
		)
	}

	t.Run("Succeeds", func(t *testing.T) {
		handler, err := newHandler(ResponseTemplate)

		assert.NilError(t, err)
		assert.Equal(t, testSiteTitle, handler.api.SiteTitle)
		assert.Equal(t, testUnsubscribeAddress, handler.mailto.UnsubscribeAddr)
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
