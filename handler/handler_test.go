package handler

import (
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"gotest.tools/assert"
)

type testSubscribeHandler struct{}
type testVerifyHandler struct{}

func (h testSubscribeHandler) HandleRequest() {
}

func (h testVerifyHandler) HandleRequest() {
}

type fixture struct {
	req events.APIGatewayV2HTTPRequest
	h   LambdaHandler
}

func newFixture() *fixture {
	return &fixture{
		h: LambdaHandler{
			SubscribeHandler: testSubscribeHandler{},
			VerifyHandler:    testVerifyHandler{},
		},
	}
}

func TestReturnsDefaultResponseLocationUntilImplemented(t *testing.T) {
	f := newFixture()
	f.req.RouteKey = "email"
	f.req.RawPath = "/email/subscribe"
	response, err := f.h.HandleApiRequest(f.req)

	assert.NilError(t, err)
	assert.Equal(t, response.StatusCode, http.StatusSeeOther)
	assert.Equal(t, response.Headers["Location"], defaultResponseLocation)
}
