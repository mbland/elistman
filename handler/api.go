package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"text/template"

	"github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/ops"
)

type RedirectMap map[ops.OperationResult]string

type apiHandler struct {
	SiteTitle        string
	Agent            ops.SubscriptionAgent
	Redirects        RedirectMap
	responseTemplate *template.Template
	log              *log.Logger
}

func newApiHandler(
	emailDomain string,
	siteTitle string,
	agent ops.SubscriptionAgent,
	paths RedirectPaths,
	responseTemplate string,
	logger *log.Logger,
) (handler *apiHandler, err error) {
	var resTmpl *template.Template
	if resTmpl, err = initResponseBodyTemplate(responseTemplate); err != nil {
		return
	}

	fullUrl := func(path string) string {
		return "https://" + emailDomain + "/" + path
	}

	return &apiHandler{
		siteTitle,
		agent,
		RedirectMap{
			ops.Invalid:           fullUrl(paths.Invalid),
			ops.AlreadySubscribed: fullUrl(paths.AlreadySubscribed),
			ops.VerifyLinkSent:    fullUrl(paths.VerifyLinkSent),
			ops.Subscribed:        fullUrl(paths.Subscribed),
			ops.NotSubscribed:     fullUrl(paths.NotSubscribed),
			ops.Unsubscribed:      fullUrl(paths.Unsubscribed),
		},
		resTmpl,
		logger,
	}, nil
}

type responseTemplateParams struct {
	Title     string
	SiteTitle string
	Body      string
}

type errorWithStatus struct {
	HttpStatus int
	Message    string
}

func (err *errorWithStatus) Error() string {
	return err.Message
}

func initResponseBodyTemplate(
	bodyTmpl string,
) (tmpl *template.Template, err error) {
	builder := &strings.Builder{}
	params := &responseTemplateParams{}

	if tmpl, err = template.New("responseBody").Parse(bodyTmpl); err != nil {
		err = fmt.Errorf("parsing response body template failed: %s", err)
	} else if err = tmpl.Execute(builder, params); err != nil {
		tmpl = nil
		err = fmt.Errorf("executing response body template failed: %s", err)
	}
	return
}

func (h *apiHandler) HandleEvent(
	ctx context.Context, origReq *events.APIGatewayV2HTTPRequest,
) (res *events.APIGatewayV2HTTPResponse) {
	req, err := newApiRequest(origReq)

	if err == nil {
		res, err = h.handleApiRequest(ctx, req)
	}

	if err != nil {
		res = h.errorResponse(err)
	}
	logApiResponse(h.log, origReq, res, err)
	return
}

func (h *apiHandler) addResponseBody(
	res *events.APIGatewayV2HTTPResponse, body string,
) {
	httpStatus := res.StatusCode
	title := fmt.Sprintf("%d %s", httpStatus, http.StatusText(httpStatus))
	params := &responseTemplateParams{title, h.SiteTitle, body}
	builder := &strings.Builder{}

	if err := h.responseTemplate.Execute(builder, params); err != nil {
		// This should never happen, but if it does, fall back to plain text.
		h.log.Printf("ERROR adding HTML response body: %s: %+v", err, params)
		res.Headers["content-type"] = "text/plain; charset=utf-8"
		res.Body = fmt.Sprintf("%s - %s\n\n%s\n", title, h.SiteTitle, body)
	} else {
		res.Headers["content-type"] = "text/html; charset=utf-8"
		res.Body = builder.String()
	}
}

func (h *apiHandler) errorResponse(err error) *events.APIGatewayV2HTTPResponse {
	res := &events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusInternalServerError,
		Headers:    map[string]string{},
	}
	if apiErr, ok := err.(*errorWithStatus); ok {
		res.StatusCode = apiErr.HttpStatus
	}

	body := "<p>There was a problem on our end; " +
		"please try again in a few minutes.</p>\n"
	h.addResponseBody(res, body)
	return res
}

func logApiResponse(
	log *log.Logger,
	req *events.APIGatewayV2HTTPRequest,
	res *events.APIGatewayV2HTTPResponse,
	err error,
) {
	reqId := req.RequestContext.RequestID
	desc := req.RequestContext.HTTP
	errMsg := ""

	if err != nil {
		errMsg = ": " + err.Error()
	}

	log.Printf(`%s: %s "%s %s %s" %d%s`,
		reqId,
		desc.SourceIP, desc.Method, desc.Path, desc.Protocol, res.StatusCode,
		errMsg,
	)
}

func newApiRequest(req *events.APIGatewayV2HTTPRequest) (*apiRequest, error) {
	contentType, foundContentType := req.Headers["content-type"]
	body := req.Body

	// This accounts for differences in HTTP Header casing between running
	// bin/smoke-test.sh against a `sam local` server and the prod deployment.
	//
	// `curl` will send "Content-Type" to the local, unencrypted, HTTP/1.1
	// server, which doesn't fully emulate a production API Gateway instance. It
	// will send "content-type" to the encrypted HTTP/2 prod deployment.
	//
	// HTTP headers are supposed to be case insensitive. HTTP/2 headers MUST be
	// lowercase:
	//
	// - Are HTTP headers case-sensitive?: https://stackoverflow.com/a/41169947
	//   - HTTP/1.1: https://www.rfc-editor.org/rfc/rfc7230#section-3.2
	//   - HTTP/2:   https://www.rfc-editor.org/rfc/rfc7540#section-8.1.2
	// - HTTP/2 Header Casing: https://blog.yaakov.online/http-2-header-casing/
	//
	// Also, the "Payload format version" of "Working with AWS Lambda proxy
	// integrations for HTTP APIs" explictly states that "Header names are
	// lowercased."
	//
	// - https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html
	if !foundContentType {
		contentType = req.Headers["Content-Type"]
	}

	// For some reason, the prod API Gateway will base64 encode POST body
	// payloads. The `sam-local` server will not. Either way, it's good to do
	// the right thing based on the value of this flag.
	if req.IsBase64Encoded {
		if decoded, err := base64.StdEncoding.DecodeString(body); err != nil {
			return nil, fmt.Errorf("failed to base64 decode body: %s", err)
		} else {
			body = string(decoded)
		}
	}

	return &apiRequest{
		req.RequestContext.RequestID,
		req.RawPath,
		req.RequestContext.HTTP.Method,
		contentType,
		req.PathParameters,
		body,
	}, nil
}

func (h *apiHandler) handleApiRequest(
	ctx context.Context, req *apiRequest,
) (*events.APIGatewayV2HTTPResponse, error) {
	res := &events.APIGatewayV2HTTPResponse{Headers: map[string]string{}}
	res.Headers["content-type"] = "text/plain; charset=utf-8"

	if op, err := parseApiRequest(req); err != nil {
		return h.respondToParseError(res, err)
	} else if result, err := h.performOperation(ctx, req.Id, op); err != nil {
		return nil, err
	} else if op.OneClick {
		res.StatusCode = http.StatusOK
	} else if redirect, ok := h.Redirects[result]; !ok {
		return nil, fmt.Errorf("no redirect for op result: %s", result)
	} else {
		res.StatusCode = http.StatusSeeOther
		res.Headers["location"] = redirect
	}
	return res, nil
}

func (h *apiHandler) respondToParseError(
	response *events.APIGatewayV2HTTPResponse, err error,
) (*events.APIGatewayV2HTTPResponse, error) {
	if !errors.Is(err, ErrUserInput) {
		response.StatusCode = http.StatusBadRequest
		body := "<p>Parsing the request failed:</p>\n" +
			"<pre>\n" + template.HTMLEscapeString(err.Error()) + "\n</pre>\n" +
			"<p>Please correct the request and try again.</p>"
		h.addResponseBody(response, body)
	} else if redirect, ok := h.Redirects[ops.Invalid]; !ok {
		return nil, errors.New("no redirect for invalid operation")
	} else {
		response.StatusCode = http.StatusSeeOther
		response.Headers["location"] = redirect
	}
	return response, nil
}

func (h *apiHandler) performOperation(
	ctx context.Context, requestId string, op *eventOperation,
) (result ops.OperationResult, err error) {
	switch op.Type {
	case Subscribe:
		result, err = h.Agent.Subscribe(ctx, op.Email)
	case Verify:
		result, err = h.Agent.Verify(ctx, op.Email, op.Uid)
	case Unsubscribe:
		result, err = h.Agent.Unsubscribe(ctx, op.Email, op.Uid)
	default:
		err = fmt.Errorf("can't handle operation type: %s", op.Type)
	}
	logOperationResult(h.log, requestId, op, result, err)

	if opErr, ok := err.(*ops.OperationErrorExternal); ok {
		err = &errorWithStatus{http.StatusBadGateway, opErr.Error()}
	}
	return result, err
}

func logOperationResult(
	log *log.Logger,
	requestId string,
	op *eventOperation,
	result ops.OperationResult,
	err error,
) {
	prefix := "result"
	errMsg := ""
	if err != nil {
		prefix = "ERROR"
		errMsg = ": " + err.Error()
	}
	log.Printf("%s: %s: %s: %s%s", requestId, prefix, op, result, errMsg)
}
