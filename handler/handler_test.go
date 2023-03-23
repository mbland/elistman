package handler

import (
	"fmt"
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
	e Event
	h LambdaHandler
}

func newFixture() *fixture {
	return &fixture{
		h: LambdaHandler{
			SubscribeHandler: testSubscribeHandler{},
			VerifyHandler:    testVerifyHandler{},
		},
	}
}

func (f *fixture) handleApiRequest(
	data []byte,
) (events.APIGatewayV2HTTPResponse, error) {
	if err := f.e.UnmarshalJSON(data); err != nil {
		return events.APIGatewayV2HTTPResponse{}, err
	} else if f.e.Type != ApiRequest {
		return events.APIGatewayV2HTTPResponse{},
			fmt.Errorf("not an API request: %s", f.e.Type)
	}
	response, err := f.h.HandleEvent(f.e)
	return response.(events.APIGatewayV2HTTPResponse), err
}

func (f *fixture) handleMailtoEvent(data []byte) (any, error) {
	if err := f.e.UnmarshalJSON(data); err != nil {
		return nil, err
	} else if f.e.Type != MailtoEvent {
		return nil, fmt.Errorf("not a mailto event: %s", f.e.Type)
	}
	return f.h.HandleEvent(f.e)
}

func TestUnexpectedEvent(t *testing.T) {
	f := newFixture()

	response, err := f.h.HandleEvent(f.e)

	assert.ErrorContains(t, err, `unexpected event: {Type:Null event`)
	assert.Equal(t, nil, response)
}

func TestApiRequestReturnsDefaultResponseLocationUntilImplemented(t *testing.T) {
	f := newFixture()

	response, err := f.handleApiRequest([]byte(`{"rawPath": "/subscribe"}`))

	assert.NilError(t, err)
	assert.Equal(t, response.StatusCode, http.StatusSeeOther)
	assert.Equal(t, response.Headers["Location"], defaultResponseLocation)
}

func TestMailtoEventDoesNothingUntilImplemented(t *testing.T) {
	f := newFixture()

	response, err := f.handleMailtoEvent([]byte(`{
		"Records": [{ "ses": { "mail": { "commonHeaders": {
			"to": [ "unsubscribe@mike-bland.com" ],
			"subject": "foo@bar.com UID"
		}}}}]
	}`))

	assert.NilError(t, err)
	assert.Equal(t, nil, response)
}
