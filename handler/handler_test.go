package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"gotest.tools/assert"
)

type fixture struct {
	ctx context.Context
	req events.APIGatewayV2HTTPRequest
	h   LambdaHandler
}

func TestReturnsDefaultResponseLocationUntilImplemented(t *testing.T) {
	f := fixture{}
	f.req.RawPath = "/email/subscribe"
	response, err := f.h.HandleRequest(f.ctx, f.req)

	assert.NilError(t, err)
	assert.Equal(t, response.StatusCode, http.StatusSeeOther)
	assert.Equal(t, response.Headers["Location"], defaultResponseLocation)
}
