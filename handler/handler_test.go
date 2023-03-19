package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"gotest.tools/assert"
)

type testSubscribeHandler struct{}
type testVerifyHandler struct{}

func (h testSubscribeHandler) HandleRequest(ctx context.Context) {
}

func (h testVerifyHandler) HandleRequest(ctx context.Context) {
}

type fixture struct {
	ctx context.Context
	req events.APIGatewayV2HTTPRequest
	h   LambdaHandler
}

func newFixture() *fixture {
	return &fixture{
		ctx: context.TODO(),
		h: LambdaHandler{
			SubscribeHandler: testSubscribeHandler{},
			VerifyHandler:    testVerifyHandler{},
		},
	}
}

func TestReturnsDefaultResponseLocationUntilImplemented(t *testing.T) {
	f := newFixture()
	f.req.RawPath = "/email/subscribe"
	response, err := f.h.HandleRequest(f.ctx, f.req)

	assert.NilError(t, err)
	assert.Equal(t, response.StatusCode, http.StatusSeeOther)
	assert.Equal(t, response.Headers["Location"], defaultResponseLocation)
}
