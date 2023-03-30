package handler

import (
	"net/http"
	"testing"

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
	e  Event
	ta *testAgent
	h  *Handler
}

func newFixture() *fixture {
	ta := &testAgent{}
	return &fixture{
		ta: ta,
		h: NewHandler(
			"mike-bland.com",
			ta,
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

func TestHandleEvent(t *testing.T) {
	t.Run("IgnoresUnexpectedEvent", func(t *testing.T) {
		f := newFixture()

		response, err := f.h.HandleEvent(&f.e)

		assert.NilError(t, err)
		assert.Equal(t, nil, response)
	})
}

func TestSubscribeRequest(t *testing.T) {
	t.Run("Successful", func(t *testing.T) {
		t.Skip("not yet implemented")
		f := newFixture()
		f.ta.ReturnValue = ops.Subscribed

		response, err := f.h.handleApiRequest(&apiRequest{
			RawPath:     "/subscribe",
			ContentType: "application/x-www-form-urlencoded",
			Body:        "email=mbland%40acm.org",
		})

		assert.NilError(t, err)
		assert.Equal(t, f.ta.Email, "mbland@acm.org")
		assert.Equal(t, response.StatusCode, http.StatusSeeOther)
		assert.Equal(
			t, response.Headers["Location"], f.h.Redirects[ops.Subscribed],
		)
	})

	t.Run("ReturnsInvalidRequestIfParsingFails", func(t *testing.T) {
		f := newFixture()

		response, err := f.h.handleApiRequest(&apiRequest{
			RawPath:     "/subscribe",
			ContentType: "application/x-www-form-urlencoded",
			Body:        "email=mbland%20acm.org",
		})

		assert.NilError(t, err)
		assert.Equal(t, f.ta.Email, "")
		assert.Equal(t, response.StatusCode, http.StatusSeeOther)
		assert.Equal(
			t, response.Headers["Location"], f.h.Redirects[ops.Invalid],
		)
	})
}

func TestMailtoEventDoesNothingUntilImplemented(t *testing.T) {
	f := newFixture()

	err := f.h.handleMailtoEvent(&mailtoEvent{
		To:      []string{"unsubscribe@mike-bland.com"},
		Subject: "foo@bar.com UID",
	})

	assert.NilError(t, err)
}
