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
	"github.com/mbland/elistman/ops"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

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

type apiHandlerFixture struct {
	agent   *testAgent
	logs    *strings.Builder
	handler *apiHandler
}

func newApiHandlerFixture() *apiHandlerFixture {
	logs, logger := testLogger()
	agent := &testAgent{}
	handler, err := newApiHandler(
		testEmailDomain,
		testSiteTitle,
		agent,
		testRedirects,
		ResponseTemplate,
		logger,
	)

	if err != nil {
		panic(err.Error())
	}
	return &apiHandlerFixture{agent, logs, handler}
}

func TestNewApiHandler(t *testing.T) {
	f := newApiHandlerFixture()

	t.Run("SetsBasicFields", func(t *testing.T) {
		assert.Equal(t, testSiteTitle, f.handler.SiteTitle)
		assert.Assert(t, f.handler.responseTemplate != nil)
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

		assert.DeepEqual(t, expected, f.handler.Redirects)
	})

	t.Run("ReturnsErrorIfTemplateFailsToParse", func(t *testing.T) {
		tmpl := "{{.Bogus}}"

		handler, err := newApiHandler(
			testEmailDomain,
			testSiteTitle,
			&testAgent{},
			testRedirects,
			tmpl,
			&log.Logger{},
		)

		assert.Assert(t, is.Nil(handler))
		assert.ErrorContains(t, err, "response body template failed")
	})
}

func TestAddResponseBody(t *testing.T) {
	const body = "<p>This is only a test</p>"

	t.Run("AddsHtmlBody", func(t *testing.T) {
		f := newApiHandlerFixture()
		res := apiGatewayResponse(http.StatusOK)

		f.handler.addResponseBody(res, body)

		assert.Equal(t, res.Headers["content-type"], "text/html; charset=utf-8")
		assert.Assert(t, is.Contains(res.Body, body))
		assert.Assert(t, is.Contains(res.Body, "200 OK - "+testSiteTitle))
	})

	t.Run("FallsBackToTextBodyOnError", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.handler.responseTemplate = template.Must(
			template.New("bogus").Parse("{{.Bogus}}"),
		)
		res := apiGatewayResponse(http.StatusOK)

		f.handler.addResponseBody(res, body)

		assert.Equal(t, res.Headers["content-type"], "text/plain; charset=utf-8")
		assert.Assert(t, is.Contains(res.Body, "This is only a test"))
		assert.Assert(t, is.Contains(res.Body, "200 OK - "+testSiteTitle))
		expected := "ERROR adding HTML response body:"
		assert.Assert(t, is.Contains(f.logs.String(), expected))
	})
}

func TestErrorResponse(t *testing.T) {
	f := newApiHandlerFixture()

	t.Run("ReturnInternalServerErrorByDefault", func(t *testing.T) {
		res := f.handler.errorResponse(fmt.Errorf("bad news..."))

		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
		assert.Assert(t, is.Contains(res.Body, "There was a problem on our end"))
	})

	t.Run("ReturnStatusFromError", func(t *testing.T) {
		err := &errorWithStatus{http.StatusBadGateway, "not our fault..."}

		res := f.handler.errorResponse(err)

		assert.Equal(t, res.StatusCode, http.StatusBadGateway)
		assert.Assert(t, is.Contains(res.Body, "There was a problem on our end"))
	})
}

func TestLogApiResponse(t *testing.T) {
	req := apiGatewayRequest(
		http.MethodGet, VerifyPrefix+"mbland%40acm.org/0123-456-789",
	)

	t.Run("WithoutError", func(t *testing.T) {
		logs, logger := testLogger()
		res := apiGatewayResponse(http.StatusOK)

		logApiResponse(logger, req, res, nil)

		expectedMsg := `192.168.0.1 "GET ` + VerifyPrefix +
			`mbland%40acm.org/0123-456-789 HTTP/2" 200`
		assert.Assert(t, is.Contains(logs.String(), expectedMsg))
	})

	t.Run("WithError", func(t *testing.T) {
		logs, logger := testLogger()
		res := apiGatewayResponse(http.StatusInternalServerError)

		logApiResponse(logger, req, res, errors.New("unexpected problem"))

		expectedMsg := `192.168.0.1 "GET ` + VerifyPrefix +
			`mbland%40acm.org/0123-456-789 HTTP/2" 500: unexpected problem`
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

func TestRespondToParseError(t *testing.T) {
	f := newApiHandlerFixture()

	t.Run("ReturnsBadRequestIfNotSubscribeOperation", func(t *testing.T) {
		res, err := f.handler.respondToParseError(
			apiGatewayResponse(http.StatusOK), errors.New("not a subscribe op"),
		)

		assert.NilError(t, err)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
		assert.Assert(t, is.Contains(res.Body, "not a subscribe op"))
	})

	t.Run("HtmlEscapesErrorInResponseBody", func(t *testing.T) {
		res, err := f.handler.respondToParseError(
			apiGatewayResponse(http.StatusOK),
			errors.New("mbland@<script>alert('pwned')</script>acm.org"),
		)

		assert.NilError(t, err)
		expected := "mbland@&lt;script&gt;alert(&#39;pwned&#39;)" +
			"&lt;/script&gt;acm.org"
		assert.Assert(t, is.Contains(res.Body, expected))
	})

	t.Run("ReturnsErrorIfInvalidOpRedirectIsMissing", func(t *testing.T) {
		f := newApiHandlerFixture()
		delete(f.handler.Redirects, ops.Invalid)

		res, err := f.handler.respondToParseError(
			apiGatewayResponse(http.StatusOK),
			&ParseError{SubscribeOp, "mbland acm.org"},
		)

		assert.Assert(t, is.Nil(res))
		assert.Error(t, err, "no redirect for invalid operation")
	})

	t.Run("RedirectsToInvalidOpPageIfSubscribeOp", func(t *testing.T) {
		res, err := f.handler.respondToParseError(
			apiGatewayResponse(http.StatusOK),
			&ParseError{SubscribeOp, "mbland acm.org"},
		)

		assert.NilError(t, err)
		assert.Equal(t, http.StatusSeeOther, res.StatusCode)
		assert.Equal(
			t, f.handler.Redirects[ops.Invalid], res.Headers["location"],
		)
	})
}

func TestLogOperationResult(t *testing.T) {
	op := &eventOperation{
		Type: VerifyOp, Email: "mbland@acm.org", Uid: testValidUid,
	}

	t.Run("SuccessfulResult", func(t *testing.T) {
		logs, logger := testLogger()

		logOperationResult(logger, "deadbeef", op, ops.Subscribed, nil)

		expected := "deadbeef: result: Verify: mbland@acm.org " +
			testValidUidStr + ": Subscribed"
		assert.Assert(t, is.Contains(logs.String(), expected))
	})

	t.Run("SuccessfulResult", func(t *testing.T) {
		logs, logger := testLogger()

		logOperationResult(
			logger, "deadbeef", op, ops.Subscribed, errors.New("whoops..."),
		)

		expected := "deadbeef: ERROR: Verify: mbland@acm.org " +
			testValidUidStr + ": Subscribed: whoops..."
		assert.Assert(t, is.Contains(logs.String(), expected))
	})
}

func TestPerformOperation(t *testing.T) {
	t.Run("SubscribeSucceeds", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.ReturnValue = ops.VerifyLinkSent

		result, err := f.handler.performOperation(
			"deadbeef",
			&eventOperation{Type: SubscribeOp, Email: "mbland@acm.org"},
		)

		assert.NilError(t, err)
		assert.Equal(t, ops.VerifyLinkSent, result)
		expectedLog := "deadbeef: result: Subscribe"
		assert.Assert(t, is.Contains(f.logs.String(), expectedLog))
	})

	t.Run("VerifySucceeds", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.ReturnValue = ops.Subscribed

		result, err := f.handler.performOperation(
			"deadbeef",
			&eventOperation{
				Type: VerifyOp, Email: "mbland@acm.org", Uid: testValidUid,
			},
		)

		assert.NilError(t, err)
		assert.Equal(t, ops.Subscribed, result)
		assert.Assert(t, is.Contains(f.logs.String(), "deadbeef: result: Verify"))
	})

	t.Run("UnsubscribeSucceeds", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.ReturnValue = ops.Unsubscribed

		result, err := f.handler.performOperation(
			"deadbeef",
			&eventOperation{
				Type: UnsubscribeOp, Email: "mbland@acm.org", Uid: testValidUid,
			},
		)

		assert.NilError(t, err)
		assert.Equal(t, ops.Unsubscribed, result)
		expectedLog := "deadbeef: result: Unsubscribe"
		assert.Assert(t, is.Contains(f.logs.String(), expectedLog))
	})

	t.Run("RaisesErrorIfCantHandleOpType", func(t *testing.T) {
		f := newApiHandlerFixture()

		result, err := f.handler.performOperation("deadbeef", &eventOperation{})

		assert.Equal(t, ops.Invalid, result)
		assert.ErrorContains(t, err, "can't handle operation type: Undefined")
		expectedLog := "deadbeef: ERROR: Undefined: Invalid: can't handle"
		assert.Assert(t, is.Contains(f.logs.String(), expectedLog))
	})

	t.Run("SetsErrorWithStatusIfExternalOpError", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.Error = &ops.OperationErrorExternal{Message: "not our fault..."}

		result, err := f.handler.performOperation(
			"deadbeef",
			&eventOperation{Type: SubscribeOp, Email: "mbland@acm.org"},
		)

		assert.Equal(t, ops.Invalid, result)
		expected := &errorWithStatus{http.StatusBadGateway, "not our fault..."}
		assert.DeepEqual(t, err, expected)
		expectedLog := "deadbeef: ERROR: Subscribe: mbland@acm.org: " +
			"Invalid: not our fault..."
		assert.Assert(t, is.Contains(f.logs.String(), expectedLog))
	})
}

func TestHandleApiRequest(t *testing.T) {
	// Use an unsubscribe request since it will allow us to hit every branch.
	newUnsubscribeRequest := func() *apiRequest {
		return &apiRequest{
			RequestId: "deadbeef",
			RawPath: UnsubscribePrefix + "mbland%40acm.org/" +
				testValidUidStr,
			Method:      http.MethodGet,
			ContentType: "text/plain",
			Params: map[string]string{
				"email": "mbland@acm.org",
				"uid":   testValidUidStr,
			},
		}
	}

	t.Run("Successful", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.ReturnValue = ops.Unsubscribed

		response, err := f.handler.handleApiRequest(newUnsubscribeRequest())

		assert.NilError(t, err)
		assert.Equal(t, "mbland@acm.org", f.agent.Email)
		assert.Equal(t, http.StatusSeeOther, response.StatusCode)
		expected := f.handler.Redirects[ops.Unsubscribed]
		assert.Equal(t, expected, response.Headers["location"])
	})

	t.Run("ReturnsBadRequestIfParsingFails", func(t *testing.T) {
		f := newApiHandlerFixture()
		req := newUnsubscribeRequest()
		req.Params["email"] = "mbland acm.org"

		response, err := f.handler.handleApiRequest(req)

		assert.NilError(t, err)
		assert.Equal(t, "", f.agent.Email)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("ReturnsErrorIfOperationFails", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.Error = &ops.OperationErrorExternal{Message: "not our fault..."}

		response, err := f.handler.handleApiRequest(newUnsubscribeRequest())

		expected := &errorWithStatus{http.StatusBadGateway, "not our fault..."}
		assert.DeepEqual(t, expected, err)
		assert.Assert(t, is.Nil(response))
	})

	t.Run("ReturnsHttp200IfOneClickUnsubscribe", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.ReturnValue = ops.Unsubscribed
		req := newUnsubscribeRequest()
		req.Method = http.MethodPost
		req.ContentType = "application/x-www-form-urlencoded"
		req.Body = "List-Unsubscribe=One-Click"

		response, err := f.handler.handleApiRequest(req)

		assert.NilError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})

	t.Run("ReturnsErrorIfNoRedirectForOpResult", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.ReturnValue = ops.Unsubscribed
		delete(f.handler.Redirects, ops.Unsubscribed)

		response, err := f.handler.handleApiRequest(newUnsubscribeRequest())

		assert.ErrorContains(t, err, "no redirect for op result: Unsubscribed")
		assert.Assert(t, is.Nil(response))
	})
}

func TestApiHandleEvent(t *testing.T) {
	req := apiGatewayRequest(http.MethodPost, SubscribePrefix)
	req.Body = "email=mbland%40acm.org"
	req.Headers = map[string]string{
		"content-type": "application/x-www-form-urlencoded",
	}

	t.Run("ReturnsErrorIfNewApiRequestFails", func(t *testing.T) {
		f := newApiHandlerFixture()
		badReq := apiGatewayRequest(http.MethodPost, SubscribePrefix)

		badReq.Body = "Definitely not base64 encoded"
		badReq.IsBase64Encoded = true

		res := f.handler.HandleEvent(badReq)

		assert.Assert(t, res != nil)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		expectedLog := "500: failed to base64 decode body"
		assert.Assert(t, is.Contains(f.logs.String(), expectedLog))
	})

	t.Run("ReturnsErrorIfHandleApiRequestFails", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.Error = &ops.OperationErrorExternal{
			Message: "db operation failed",
		}

		res := f.handler.HandleEvent(req)

		assert.Assert(t, res != nil)
		assert.Equal(t, http.StatusBadGateway, res.StatusCode)
		assert.Assert(
			t, is.Contains(f.logs.String(), "502: db operation failed"),
		)
	})

	t.Run("Succeeds", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.ReturnValue = ops.VerifyLinkSent

		res := f.handler.HandleEvent(req)

		assert.Assert(t, res != nil)
		assert.Equal(t, http.StatusSeeOther, res.StatusCode)
		assert.Assert(t, strings.HasSuffix(f.logs.String(), " 303\n"))
	})
}
