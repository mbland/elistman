package handler

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/uuid"
	"gotest.tools/assert"
)

type testAgent struct{}

func (h testAgent) Subscribe(email string) (bool, error) {
	return true, nil
}

func (h testAgent) Verify(email string, uid uuid.UUID) (bool, error) {
	return true, nil
}

func (h testAgent) Unsubscribe(email string, uid uuid.UUID) (bool, error) {
	return true, nil
}

type fixture struct {
	e Event
	h Handler
}

func newFixture() *fixture {
	return &fixture{
		h: Handler{
			Agent: testAgent{},
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

func TestIgnoreUnexpectedEvent(t *testing.T) {
	f := newFixture()

	response, err := f.h.HandleEvent(f.e)

	assert.NilError(t, err)
	assert.Equal(t, nil, response)
}

func TestApiRequestReturnsDefaultResponseLocationUntilImplemented(t *testing.T) {
	f := newFixture()

	response, err := f.handleApiRequest([]byte(`{
		"rawPath": "/subscribe",
		"pathParameters": {
			"email": "mbland%40acm.org",
			"uid": "00000000-1111-2222-3333-444444444444"
		}
	}`))

	assert.NilError(t, err)
	assert.Equal(t, response.StatusCode, http.StatusBadRequest)
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
