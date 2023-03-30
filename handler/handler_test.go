package handler

import (
	"fmt"
	"net/http"
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

type fixture struct {
	e Event
	h *Handler
}

func newFixture() *fixture {
	return &fixture{
		h: NewHandler(
			"mike-bland.com",
			&testAgent{},
			RedirectPaths{
				"invalid",
				"already-subscribed",
				"verify-link-sent",
				"subscribed",
				"not-subscribed",
				"unsubscribed",
			}),
	}
}

func (f *fixture) handleApiRequest(
	data []byte,
) (*events.APIGatewayV2HTTPResponse, error) {
	if err := f.e.UnmarshalJSON(data); err != nil {
		return nil, err
	} else if f.e.Type != ApiRequest {
		return nil, fmt.Errorf("not an API request: %s", f.e.Type)
	}
	response, err := f.h.HandleEvent(&f.e)
	return response.(*events.APIGatewayV2HTTPResponse), err
}

func (f *fixture) handleMailtoEvent(data []byte) (any, error) {
	if err := f.e.UnmarshalJSON(data); err != nil {
		return nil, err
	} else if f.e.Type != MailtoEvent {
		return nil, fmt.Errorf("not a mailto event: %s", f.e.Type)
	}
	return f.h.HandleEvent(&f.e)
}

func TestIgnoreUnexpectedEvent(t *testing.T) {
	f := newFixture()

	response, err := f.h.HandleEvent(&f.e)

	assert.NilError(t, err)
	assert.Equal(t, nil, response)
}

func TestApiRequestReturnsInvalidRequestLocationUntilImplemented(t *testing.T) {
	f := newFixture()

	response, err := f.handleApiRequest([]byte(`{
		"rawPath": "/subscribe",
		"body": "email=mbland%40acm.org"
	}`))

	assert.NilError(t, err)
	assert.Equal(t, response.StatusCode, http.StatusSeeOther)
	assert.Equal(t, response.Headers["Location"], f.h.Redirects[ops.Invalid])
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
