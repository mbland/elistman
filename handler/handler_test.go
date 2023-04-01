package handler

import (
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/google/uuid"
	"github.com/mbland/elistman/ops"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
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

const testEmailDomain = "mike-bland.com"
const testUnsubscribeAddress = "unsubscribe@" + testEmailDomain

var testRedirects = RedirectPaths{
	Invalid:           "invalid",
	AlreadySubscribed: "already-subscribed",
	VerifyLinkSent:    "verify-link-sent",
	Subscribed:        "subscribed",
	NotSubscribed:     "not-subscribed",
	Unsubscribed:      "unsubscribed",
}

func newFixture() *fixture {
	ta := &testAgent{}
	return &fixture{ta: ta, h: NewHandler(testEmailDomain, ta, testRedirects)}
}

func TestNewHandler(t *testing.T) {
	f := newFixture()

	t.Run("SetsUnsubscribeAddress", func(t *testing.T) {
		assert.Equal(t, testUnsubscribeAddress, f.h.UnsubscribeAddr)
	})

	t.Run("SetsRedirectMap", func(t *testing.T) {
		fullUrl := func(path string) string {
			return "https://" + testEmailDomain + "/" + path
		}
		expected := RedirectMap{
			ops.Invalid:           fullUrl(testRedirects.Invalid),
			ops.AlreadySubscribed: fullUrl(testRedirects.AlreadySubscribed),
			ops.VerifyLinkSent:    fullUrl(testRedirects.VerifyLinkSent),
			ops.Subscribed:        fullUrl(testRedirects.Subscribed),
			ops.NotSubscribed:     fullUrl(testRedirects.NotSubscribed),
			ops.Unsubscribed:      fullUrl(testRedirects.Unsubscribed),
		}

		assert.DeepEqual(t, expected, f.h.Redirects)
	})
}

func TestNewApiRequest(t *testing.T) {
	const requestId = "deadbeef"
	const rawPath = UnsubscribePrefix + "/mbland%40acm.org/0123-456-789"
	const contentType = "application/x-www-form-urlencoded; charset=utf-8"
	const body = "List-Unsubscribe=One-Click"
	pathParams := map[string]string{
		"email": "mbland@acm.org", "uid": "0123-456-789",
	}

	newReq := func() *events.APIGatewayV2HTTPRequest {
		return &events.APIGatewayV2HTTPRequest{
			RawPath: rawPath,
			RequestContext: events.APIGatewayV2HTTPRequestContext{
				RequestID: requestId,
				HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
					Method: http.MethodPost,
				},
			},
			Headers:        map[string]string{"content-type": contentType},
			PathParameters: pathParams,
			Body:           body,
		}
	}

	expectedReq := &apiRequest{
		requestId, rawPath, http.MethodPost, contentType, pathParams, body,
	}

	t.Run("Succeeds", func(t *testing.T) {
		req, err := newApiRequest(newReq())

		assert.NilError(t, err)
		assert.DeepEqual(t, expectedReq, req)
	})

	t.Run("ParsesUppercaseContentType", func(t *testing.T) {
		awsReq := newReq()
		delete(awsReq.Headers, "content-type")
		awsReq.Headers["Content-Type"] = contentType

		req, err := newApiRequest(awsReq)

		assert.NilError(t, err)
		assert.Equal(t, contentType, req.ContentType)
	})

	t.Run("DecodesBase64EncodedBody", func(t *testing.T) {
		awsReq := newReq()
		awsReq.Body = base64.StdEncoding.EncodeToString([]byte(body))
		awsReq.IsBase64Encoded = true

		req, err := newApiRequest(awsReq)

		assert.NilError(t, err)
		assert.Equal(t, body, req.Body)
	})

	t.Run("ErrorsIfBase64DecodingFails", func(t *testing.T) {
		awsReq := newReq()
		// Set to true without actually encoding the body.
		awsReq.IsBase64Encoded = true

		req, err := newApiRequest(awsReq)

		assert.ErrorContains(t, err, "failed to base64 decode body: ")
		assert.Assert(t, is.Nil(req))
	})
}

func TestNewMailtoEvent(t *testing.T) {
	from := []string{"mbland@acm.org"}
	to := []string{testUnsubscribeAddress}
	subject := from[0] + " 0123-456-789"
	const messageId = "deadbeef"
	const spfVerdict = "PASS"
	const dkimVerdict = "PASS"
	const spamVerdict = "PASS"
	const virusVerdict = "PASS"
	const dmarcVerdict = "PASS"
	const dmarcPolicy = "REJECT"

	sesEvent := &events.SimpleEmailService{
		Mail: events.SimpleEmailMessage{
			MessageID: messageId,
			CommonHeaders: events.SimpleEmailCommonHeaders{
				From:    from,
				To:      to,
				Subject: subject,
			},
		},
		Receipt: events.SimpleEmailReceipt{
			SPFVerdict:   events.SimpleEmailVerdict{Status: spfVerdict},
			DKIMVerdict:  events.SimpleEmailVerdict{Status: dkimVerdict},
			SpamVerdict:  events.SimpleEmailVerdict{Status: spamVerdict},
			VirusVerdict: events.SimpleEmailVerdict{Status: virusVerdict},
			DMARCVerdict: events.SimpleEmailVerdict{Status: dmarcVerdict},
			DMARCPolicy:  dmarcPolicy,
		},
	}

	expected := &mailtoEvent{
		from, to, subject, messageId,
		spfVerdict, dkimVerdict, spamVerdict, virusVerdict,
		dmarcVerdict, dmarcPolicy,
	}

	assert.DeepEqual(t, expected, newMailtoEvent(sesEvent))
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
			t, response.Headers["location"], f.h.Redirects[ops.Invalid],
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
