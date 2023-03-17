package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"gotest.tools/assert"
)

type fixture struct {
	ctx   context.Context
	event events.APIGatewayV2HTTPRequest
}

func TestReturnsDefaultResponseLocationUntilImplemented(t *testing.T) {
	f := fixture{}
	response, err := Handler(f.ctx, f.event)

	assert.NilError(t, err)
	assert.Equal(t, response.StatusCode, http.StatusSeeOther)
	assert.Equal(t, response.Headers["Location"], defaultResponseLocation)
}
