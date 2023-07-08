//go:build small_tests || all_tests

package handler

import (
	"context"
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
	"github.com/mbland/elistman/testutils"
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
	logs    *testutils.Logs
	handler *apiHandler
	ctx     context.Context
}

func newApiHandlerFixture() *apiHandlerFixture {
	logs := &testutils.Logs{}
	agent := &testAgent{}
	handler, err := newApiHandler(
		testEmailDomain,
		testSiteTitle,
		agent,
		testRedirects,
		ResponseTemplate,
		logs.NewLogger(),
	)

	if err != nil {
		panic(err.Error())
	}
	return &apiHandlerFixture{agent, logs, handler, context.Background()}
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
		f.logs.AssertContains(t, "ERROR adding HTML response body:")
	})
}

// newOpsErrExternal returns an error wrapping ops.ErrExternal.
//
// Returning a wrapped error ensures that the code under test uses errors.Is to
// detect the presence of ops.ErrExternal instead of a type assertion.
func newOpsErrExternal(msg string) error {
	return fmt.Errorf("%w: %s", ops.ErrExternal, msg)
}

// newBadGatewayError returns an errorWithStatus with http.StatusBadGateway.
//
// apiHandler.performOperation returns such a value when it detects a wrapped
// ops.ErrExternal error. For that reason, this test helper uses the
// newOpsErrExternal test helper to generate the error message for the returned
// object.
func newBadGatewayError(msg string) error {
	return &errorWithStatus{
		http.StatusBadGateway, newOpsErrExternal(msg).Error(),
	}
}

func TestErrorResponse(t *testing.T) {
	f := newApiHandlerFixture()

	t.Run("ReturnInternalServerErrorByDefault", func(t *testing.T) {
		res := f.handler.errorResponse(fmt.Errorf("bad news..."))

		assert.Equal(t, res.StatusCode, http.StatusInternalServerError)
		assert.Assert(t, is.Contains(res.Body, "There was a problem on our end"))
	})

	t.Run("ReturnStatusFromError", func(t *testing.T) {
		err := fmt.Errorf(
			"wrapped to ensure errorResponse uses errors.As: %w",
			newBadGatewayError("not our fault..."),
		)

		res := f.handler.errorResponse(err)

		assert.Equal(t, res.StatusCode, http.StatusBadGateway)
		assert.Assert(t, is.Contains(res.Body, "There was a problem on our end"))
	})
}

func TestLogApiResponse(t *testing.T) {
	req := apiGatewayRequest(
		http.MethodGet, ops.ApiPrefixVerify+"mbland@acm.org/0123-456-789",
	)

	t.Run("WithoutError", func(t *testing.T) {
		logs := testutils.Logs{}
		res := apiGatewayResponse(http.StatusOK)

		logApiResponse(logs.NewLogger(), req, res, nil)

		expectedMsg := `192.168.0.1 "GET ` + ops.ApiPrefixVerify +
			`mbland@acm.org/0123-456-789 HTTP/2" 200`
		logs.AssertContains(t, expectedMsg)
	})

	t.Run("WithError", func(t *testing.T) {
		logs, logger := testutils.NewLogs()
		res := apiGatewayResponse(http.StatusInternalServerError)

		logApiResponse(logger, req, res, errors.New("unexpected problem"))

		expectedMsg := `192.168.0.1 "GET ` + ops.ApiPrefixVerify +
			`mbland@acm.org/0123-456-789 HTTP/2" 500: unexpected problem`
		logs.AssertContains(t, expectedMsg)
	})
}

func TestNewApiRequest(t *testing.T) {
	const requestId = "deadbeef"
	const rawPath = ops.ApiPrefixUnsubscribe + "/mbland@acm.org/0123-456-789"
	const contentType = "application/x-www-form-urlencoded; charset=utf-8"
	const body = "List-Unsubscribe=One-Click"
	pathParams := map[string]string{
		"email": "mbland@acm.org", "uid": "0123-456-789",
	}

	newReq := func() *events.APIGatewayProxyRequest {
		return &events.APIGatewayProxyRequest{
			HTTPMethod: http.MethodPost,
			RequestContext: events.APIGatewayProxyRequestContext{
				RequestID:    requestId,
				ResourcePath: rawPath,
				Identity:     events.APIGatewayRequestIdentity{},
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
	userInputError := fmt.Errorf("%w: PEBKAC", ErrUserInput)

	t.Run("ReturnsBadRequestIfNotErrUserInput", func(t *testing.T) {
		res, err := f.handler.respondToParseError(
			apiGatewayResponse(http.StatusOK), errors.New("not a PEBKAC"),
		)

		assert.NilError(t, err)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode)
		assert.Assert(t, is.Contains(res.Body, "not a PEBKAC"))
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
			apiGatewayResponse(http.StatusOK), userInputError,
		)

		assert.Assert(t, is.Nil(res))
		assert.Error(t, err, "no redirect for invalid operation")
	})

	t.Run("RedirectsToInvalidOpPageIfBadSubscribeInput", func(t *testing.T) {
		res, err := f.handler.respondToParseError(
			apiGatewayResponse(http.StatusOK), userInputError,
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
		Type: Verify, Email: "mbland@acm.org", Uid: testValidUid,
	}

	t.Run("SuccessfulResult", func(t *testing.T) {
		logs, logger := testutils.NewLogs()

		logOperationResult(logger, "deadbeef", op, ops.Subscribed, nil)

		expected := "deadbeef: result: Verify: mbland@acm.org " +
			testValidUidStr + ": Subscribed"
		logs.AssertContains(t, expected)
	})

	t.Run("SuccessfulResult", func(t *testing.T) {
		logs, logger := testutils.NewLogs()

		logOperationResult(
			logger, "deadbeef", op, ops.Subscribed, errors.New("whoops..."),
		)

		expected := "deadbeef: ERROR: Verify: mbland@acm.org " +
			testValidUidStr + ": Subscribed: whoops..."
		logs.AssertContains(t, expected)
	})
}

func TestPerformOperation(t *testing.T) {
	t.Run("SubscribeSucceeds", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.OpResult = ops.VerifyLinkSent

		result, err := f.handler.performOperation(
			f.ctx,
			"deadbeef",
			&eventOperation{Type: Subscribe, Email: "mbland@acm.org"},
		)

		assert.NilError(t, err)
		assert.Equal(t, ops.VerifyLinkSent, result)
		f.logs.AssertContains(t, "deadbeef: result: Subscribe")
	})

	t.Run("VerifySucceeds", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.OpResult = ops.Subscribed

		result, err := f.handler.performOperation(
			f.ctx,
			"deadbeef",
			&eventOperation{
				Type: Verify, Email: "mbland@acm.org", Uid: testValidUid,
			},
		)

		assert.NilError(t, err)
		assert.Equal(t, ops.Subscribed, result)
		f.logs.AssertContains(t, "deadbeef: result: Verify")
	})

	t.Run("UnsubscribeSucceeds", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.OpResult = ops.Unsubscribed

		result, err := f.handler.performOperation(
			f.ctx,
			"deadbeef",
			&eventOperation{
				Type: Unsubscribe, Email: "mbland@acm.org", Uid: testValidUid,
			},
		)

		assert.NilError(t, err)
		assert.Equal(t, ops.Unsubscribed, result)
		f.logs.AssertContains(t, "deadbeef: result: Unsubscribe")
	})

	t.Run("RaisesErrorIfCantHandleOpType", func(t *testing.T) {
		f := newApiHandlerFixture()

		result, err := f.handler.performOperation(
			f.ctx, "deadbeef", &eventOperation{},
		)

		assert.Equal(t, ops.Invalid, result)
		assert.ErrorContains(t, err, "can't handle operation type: Undefined")
		expectedLog := "deadbeef: ERROR: Undefined: Invalid: can't handle"
		f.logs.AssertContains(t, expectedLog)
	})

	t.Run("SetsErrorWithStatusIfExternalOpError", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.Error = newOpsErrExternal("not our fault...")

		result, err := f.handler.performOperation(
			f.ctx,
			"deadbeef",
			&eventOperation{Type: Subscribe, Email: "mbland@acm.org"},
		)

		assert.Equal(t, ops.Invalid, result)
		assert.DeepEqual(t, newBadGatewayError("not our fault..."), err)
		expectedLog := "deadbeef: ERROR: Subscribe: mbland@acm.org: Invalid: "
		f.logs.AssertContains(t, expectedLog)
		f.logs.AssertContains(t, "not our fault...")
	})
}

func TestHandleApiRequest(t *testing.T) {
	// Use an unsubscribe request since it will allow us to hit every branch.
	newUnsubscribeRequest := func() *apiRequest {
		return &apiRequest{
			Id: "deadbeef",
			RawPath: ops.ApiPrefixUnsubscribe + "mbland@acm.org/" +
				testValidUidStr,
			Method:      http.MethodPost,
			ContentType: "application/x-www-form-urlencoded",
			Params: map[string]string{
				"email": "mbland@acm.org",
				"uid":   testValidUidStr,
			},
		}
	}

	t.Run("Successful", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.OpResult = ops.Unsubscribed

		response, err := f.handler.handleApiRequest(
			f.ctx, newUnsubscribeRequest(),
		)

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

		response, err := f.handler.handleApiRequest(f.ctx, req)

		assert.NilError(t, err)
		assert.Equal(t, "", f.agent.Email)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode)
	})

	t.Run("ReturnsErrorIfOperationFails", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.Error = newOpsErrExternal("not our fault...")

		response, err := f.handler.handleApiRequest(
			f.ctx, newUnsubscribeRequest(),
		)

		assert.DeepEqual(t, newBadGatewayError("not our fault..."), err)
		assert.Assert(t, is.Nil(response))
	})

	t.Run("ReturnsHttp200IfOneClickUnsubscribe", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.OpResult = ops.Unsubscribed
		req := newUnsubscribeRequest()
		req.Method = http.MethodPost
		req.ContentType = "application/x-www-form-urlencoded"
		req.Body = "List-Unsubscribe=One-Click"

		response, err := f.handler.handleApiRequest(f.ctx, req)

		assert.NilError(t, err)
		assert.Equal(t, http.StatusOK, response.StatusCode)
	})

	t.Run("ReturnsErrorIfNoRedirectForOpResult", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.OpResult = ops.Unsubscribed
		delete(f.handler.Redirects, ops.Unsubscribed)

		response, err := f.handler.handleApiRequest(
			f.ctx, newUnsubscribeRequest(),
		)

		assert.ErrorContains(t, err, "no redirect for op result: Unsubscribed")
		assert.Assert(t, is.Nil(response))
	})
}

func TestApiHandleEvent(t *testing.T) {
	req := apiGatewayRequest(http.MethodPost, ops.ApiPrefixSubscribe)
	req.Body = "email=mbland%40acm.org"
	req.Headers = map[string]string{
		"content-type": "application/x-www-form-urlencoded",
	}

	t.Run("ReturnsErrorIfNewApiRequestFails", func(t *testing.T) {
		f := newApiHandlerFixture()
		badReq := apiGatewayRequest(http.MethodPost, ops.ApiPrefixSubscribe)

		badReq.Body = "Definitely not base64 encoded"
		badReq.IsBase64Encoded = true

		res := f.handler.HandleEvent(f.ctx, badReq)

		assert.Assert(t, res != nil)
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
		f.logs.AssertContains(t, "500: failed to base64 decode body")
	})

	t.Run("ReturnsErrorIfHandleApiRequestFails", func(t *testing.T) {
		f := newApiHandlerFixture()
		const errMsg = "db operation failed"
		f.agent.Error = newOpsErrExternal(errMsg)

		res := f.handler.HandleEvent(f.ctx, req)

		assert.Assert(t, res != nil)
		assert.Equal(t, http.StatusBadGateway, res.StatusCode)
		f.logs.AssertContains(t, "502: "+newBadGatewayError(errMsg).Error())
	})

	t.Run("Succeeds", func(t *testing.T) {
		f := newApiHandlerFixture()
		f.agent.OpResult = ops.VerifyLinkSent

		res := f.handler.HandleEvent(f.ctx, req)

		assert.Assert(t, res != nil)
		assert.Equal(t, http.StatusSeeOther, res.StatusCode)
		assert.Assert(t, strings.HasSuffix(f.logs.Logs(), " 303\n"))
	})
}
