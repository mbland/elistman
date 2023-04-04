package handler

import (
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/uuid"
	"github.com/mbland/elistman/ops"
	"gotest.tools/assert"
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
	handler *Handler
}

func newFixture() *handlerFixture {
	ta := &testAgent{}
	handler, err := NewHandler(
		testEmailDomain, testSiteTitle, ta, testRedirects, ResponseTemplate,
	)

	if err != nil {
		panic(err.Error())
	}
	return &handlerFixture{agent: ta, handler: handler}
}

func captureLogs() (*strings.Builder, func()) {
	origWriter := log.Writer()
	builder := &strings.Builder{}
	log.SetOutput(builder)

	return builder, func() {
		log.SetOutput(origWriter)
	}
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

func TestHandleEvent(t *testing.T) {
	t.Run("ReturnsErrorOnUnexpectedEvent", func(t *testing.T) {
		f := newFixture()
		event := &Event{}

		response, err := f.handler.HandleEvent(event)

		assert.Equal(t, nil, response)
		expected := fmt.Sprintf(
			"unexpected event type: %s: %+v", NullEvent, event,
		)
		assert.Error(t, err, expected)
	})
}
