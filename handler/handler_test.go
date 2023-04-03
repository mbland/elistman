package handler

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"
	"text/template"

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
const testSiteTitle = "Mike Bland's blog"
const testUnsubscribeAddress = "unsubscribe@" + testEmailDomain

// const testValidUid = "00000000-1111-2222-3333-444444444444"

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
	handler, err := NewHandler(
		testEmailDomain, testSiteTitle, ta, testRedirects, ResponseTemplate,
	)

	if err != nil {
		panic(err.Error())
	}
	return &fixture{ta: ta, h: handler}
}

func TestInitResponseBodyTemplate(t *testing.T) {
	t.Run("SucceedsWithDefaultResponseTemplate", func(t *testing.T) {
		tmpl, err := initResponseBodyTemplate(ResponseTemplate)

		assert.NilError(t, err)
		assert.Assert(t, tmpl != nil)
	})

	t.Run("ErrorOnMalformedTemplate", func(t *testing.T) {
		bogusTemplate := "{{}{{bogus}}}"

		tmpl, err := initResponseBodyTemplate(bogusTemplate)

		assert.Assert(t, is.Nil(tmpl))
		assert.ErrorContains(t, err, "parsing response body template failed")
	})

	t.Run("ErrorOnTemplateWithUnexpectedParams", func(t *testing.T) {
		bogusTemplate := "{{.Bogus}}"

		tmpl, err := initResponseBodyTemplate(bogusTemplate)

		assert.Assert(t, is.Nil(tmpl))
		assert.ErrorContains(t, err, "executing response body template failed")
	})
}

func TestNewHandler(t *testing.T) {
	f := newFixture()

	t.Run("SetsBasicFields", func(t *testing.T) {
		assert.Equal(t, testUnsubscribeAddress, f.h.UnsubscribeAddr)
		assert.Equal(t, testSiteTitle, f.h.SiteTitle)
		assert.Assert(t, f.h.responseTemplate != nil)
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

	t.Run("ReturnsErrorIfTemplateFailsToParse", func(t *testing.T) {
		tmpl := "{{.Bogus}}"

		handler, err := NewHandler(
			testEmailDomain, testSiteTitle, &testAgent{}, testRedirects, tmpl,
		)

		assert.Assert(t, is.Nil(handler))
		assert.ErrorContains(t, err, "response body template failed")
	})
}

func captureLogs() (*strings.Builder, func()) {
	origWriter := log.Writer()
	builder := &strings.Builder{}
	log.SetOutput(builder)

	return builder, func() {
		log.SetOutput(origWriter)
	}
}

func apiGatewayRequest(method, path string) *events.APIGatewayV2HTTPRequest {
	return &events.APIGatewayV2HTTPRequest{
		RawPath: path,
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			RequestID: "deadbeef",
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				SourceIP: "192.168.0.1",
				Method:   method,
				Path:     path,
				Protocol: "HTTP/2",
			},
		},
	}
}

func apiGatewayResponse(status int) *events.APIGatewayV2HTTPResponse {
	return &events.APIGatewayV2HTTPResponse{
		StatusCode: status, Headers: map[string]string{},
	}
}

func TestAddResponseBody(t *testing.T) {
	const body = "<p>This is only a test</p>"
	f := newFixture()
	res := apiGatewayResponse(http.StatusOK)

	t.Run("AddsHtmlBody", func(t *testing.T) {
		f.h.addResponseBody(res, body)

		assert.Equal(t, res.Headers["content-type"], "text/html; charset=utf-8")
		assert.Assert(t, is.Contains(res.Body, body))
		assert.Assert(t, is.Contains(res.Body, "200 OK - "+testSiteTitle))
	})

	t.Run("FallsBackToTextBodyOnError", func(t *testing.T) {
		tmpl := template.Must(template.New("bogus").Parse("{{.Bogus}}"))
		f.h.responseTemplate = tmpl
		logs, teardown := captureLogs()
		defer teardown()

		f.h.addResponseBody(res, body)

		assert.Equal(t, res.Headers["content-type"], "text/plain; charset=utf-8")
		assert.Assert(t, is.Contains(res.Body, "This is only a test"))
		assert.Assert(t, is.Contains(res.Body, "200 OK - "+testSiteTitle))
		expected := "ERROR adding HTML response body:"
		assert.Assert(t, is.Contains(logs.String(), expected))
	})
}

func TestErrorResponse(t *testing.T) {
	f := newFixture()

	t.Run("ReturnInternalServerErrorByDefault", func(t *testing.T) {
		res := f.h.errorResponse(fmt.Errorf("bad news..."))

		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
		assert.Assert(t, is.Contains(res.Body, "There was a problem on our end"))
	})

	t.Run("ReturnStatusFromError", func(t *testing.T) {
		err := &errorWithStatus{http.StatusBadGateway, "not our fault..."}

		res := f.h.errorResponse(err)

		assert.Equal(t, res.StatusCode, http.StatusBadGateway)
		assert.Assert(t, is.Contains(res.Body, "There was a problem on our end"))
	})
}

func TestLogApiResponse(t *testing.T) {
	req := apiGatewayRequest(
		http.MethodGet, "/verify/mbland%40acm.org/0123-456-789",
	)

	t.Run("WithoutError", func(t *testing.T) {
		logs, teardown := captureLogs()
		defer teardown()
		res := apiGatewayResponse(http.StatusOK)

		logApiResponse(req, res, nil)

		expectedMsg := `192.168.0.1 ` +
			`"GET /verify/mbland%40acm.org/0123-456-789 HTTP/2" 200`
		assert.Assert(t, is.Contains(logs.String(), expectedMsg))
	})

	t.Run("WithError", func(t *testing.T) {
		logs, teardown := captureLogs()
		defer teardown()
		res := apiGatewayResponse(http.StatusInternalServerError)

		logApiResponse(req, res, errors.New("unexpected problem"))

		expectedMsg := `192.168.0.1 ` +
			`"GET /verify/mbland%40acm.org/0123-456-789 HTTP/2" ` +
			`500: unexpected problem`
		assert.Assert(t, is.Contains(logs.String(), expectedMsg))
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

func TestHandleApiEvent(t *testing.T) {
	req := apiGatewayRequest(http.MethodPost, "/subscribe")
	logs, teardown := captureLogs()
	defer teardown()

	req.Body = "email=mbland%40acm.org"
	req.Headers = map[string]string{
		"content-type": "application/x-www-form-urlencoded",
	}

	t.Run("ReturnsErrorIfNewApiRequestFails", func(t *testing.T) {
		f := newFixture()
		badReq := apiGatewayRequest(http.MethodPost, "/subscribe")
		defer logs.Reset()

		badReq.Body = "Definitely not base64 encoded"
		badReq.IsBase64Encoded = true

		res := f.h.handleApiEvent(badReq)

		assert.Assert(t, res != nil)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		assert.Assert(
			t, is.Contains(logs.String(), "500: failed to base64 decode body"),
		)
	})

	t.Run("ReturnsErrorIfHandleApiRequestFails", func(t *testing.T) {
		f := newFixture()
		f.ta.Error = &ops.OperationErrorExternal{Message: "db operation failed"}
		defer logs.Reset()

		res := f.h.handleApiEvent(req)

		assert.Assert(t, res != nil)
		assert.Equal(t, http.StatusBadGateway, res.StatusCode)
		assert.Assert(
			t, is.Contains(logs.String(), "502: db operation failed"),
		)
	})

	t.Run("Succeeds", func(t *testing.T) {
		f := newFixture()
		f.ta.ReturnValue = ops.VerifyLinkSent
		defer logs.Reset()

		res := f.h.handleApiEvent(req)

		assert.Assert(t, res != nil)
		assert.Equal(t, http.StatusSeeOther, res.StatusCode)
		assert.Assert(t, strings.HasSuffix(logs.String(), " 303\n"))
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

func TestHandleEvent(t *testing.T) {
	t.Run("ReturnsErrorOnUnexpectedEvent", func(t *testing.T) {
		f := newFixture()

		response, err := f.h.HandleEvent(&f.e)

		assert.Equal(t, nil, response)
		expected := fmt.Sprintf(
			"unexpected event type: %s: %+v", NullEvent, &f.e,
		)
		assert.Error(t, err, expected)
	})
}
