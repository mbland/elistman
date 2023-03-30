package handler

import (
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/uuid"
	"github.com/mbland/elistman/ops"
	"gotest.tools/assert"
)

type testApiRequest struct {
	RawPath string
	Headers map[string]string
	Body    string
}

type testMailtoEvent struct {
	From         []string
	To           []string
	Subject      string
	SpfVerdict   string
	DkimVerdict  string
	SpamVerdict  string
	VirusVerdict string
	DmarcVerdict string
	DmarcPolicy  string
}

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

func (f *fixture) handleApiRequest(
	testReq *testApiRequest,
) (*events.APIGatewayV2HTTPResponse, error) {
	f.e.Type = ApiRequest
	fReq := &f.e.ApiRequest
	fReq.RawPath = testReq.RawPath
	fReq.Headers = testReq.Headers
	fReq.Body = testReq.Body
	response, err := f.h.HandleEvent(&f.e)
	return response.(*events.APIGatewayV2HTTPResponse), err
}

func (f *fixture) handleMailtoEvent(e *testMailtoEvent) (any, error) {
	f.e.Type = MailtoEvent
	ses := events.SimpleEmailService{}
	headers := ses.Mail.CommonHeaders
	receipt := &ses.Receipt

	headers.From = e.From
	headers.To = e.To
	headers.Subject = e.Subject

	receipt.SPFVerdict.Status = e.SpfVerdict
	receipt.DKIMVerdict.Status = e.DkimVerdict
	receipt.SpamVerdict.Status = e.SpamVerdict
	receipt.VirusVerdict.Status = e.VirusVerdict
	receipt.DMARCVerdict.Status = e.DmarcVerdict
	receipt.DMARCPolicy = e.DmarcPolicy

	f.e.MailtoEvent.Records = append(
		f.e.MailtoEvent.Records, events.SimpleEmailRecord{SES: ses},
	)
	return f.h.HandleEvent(&f.e)
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

		response, err := f.handleApiRequest(&testApiRequest{
			RawPath: "/subscribe",
			Headers: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			Body: "email=mbland%40acm.org",
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

		response, err := f.handleApiRequest(&testApiRequest{
			RawPath: "/subscribe",
			Headers: map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
			},
			Body: "email=mbland%20acm.org",
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

	response, err := f.handleMailtoEvent(&testMailtoEvent{
		To:      []string{"unsubscribe@mike-bland.com"},
		Subject: "foo@bar.com UID",
	})

	assert.NilError(t, err)
	assert.Equal(t, nil, response)
}
