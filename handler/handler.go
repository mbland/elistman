package handler

import (
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

type Handler struct {
	UnsubscribeAddr  string
	SiteTitle        string
	Agent            ops.SubscriptionAgent
	Redirects        RedirectMap
	responseTemplate *template.Template
}

func NewHandler(
	emailDomain string,
	siteTitle string,
	agent ops.SubscriptionAgent,
	paths RedirectPaths,
	responseTemplate string,
) (*Handler, error) {
	fullUrl := func(path string) string {
		return "https://" + emailDomain + "/" + path
	}
	responseTmpl, err := initResponseBodyTemplate(responseTemplate)

	if err != nil {
		return nil, err
	}

	return &Handler{
		"unsubscribe@" + emailDomain,
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
		responseTmpl,
	}, nil
}

func initResponseBodyTemplate(bodyTmpl string) (*template.Template, error) {
	builder := &strings.Builder{}
	params := &ResponseTemplateParams{}

	if tmpl, err := template.New("responseBody").Parse(bodyTmpl); err != nil {
		return nil, fmt.Errorf("parsing response body template failed: %s", err)
	} else if err := tmpl.Execute(builder, params); err != nil {
		return nil, fmt.Errorf(
			"executing response body template failed: %s", err,
		)
	} else {
		return tmpl, nil
	}
}

const ResponseTemplate = `<!DOCTYPE html>
<html lang="en-us">
  <head>
	<meta charset="utf-8"/>
	<meta name="viewport" content="width=device-width, initial-scale=1"/>
	<title>{{.Title}} - {{.SiteTitle}}</title>
  </head>
  <body>
    <h1>{{.Title}}</h1>
    {{.Body}}
  </body>
</html>`

type ResponseTemplateParams struct {
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

func (h *Handler) HandleEvent(event *Event) (any, error) {
	switch event.Type {
	case ApiRequest:
		return h.handleApiEvent(&event.ApiRequest), nil
	case MailtoEvent:
		// If I understand the contract correctly, there should only ever be one
		// valid Record per event. However, we have the technology to deal
		// gracefully with the unexpected.
		errs := make([]error, len(event.MailtoEvent.Records))

		for i, record := range event.MailtoEvent.Records {
			errs[i] = h.handleMailtoEvent(newMailtoEvent(&record.SES))
		}
		return nil, errors.Join(errs...)
	}
	return nil, fmt.Errorf("unexpected event type: %s: %+v", event.Type, event)
}

func (h *Handler) handleApiEvent(
	origReq *events.APIGatewayV2HTTPRequest,
) *events.APIGatewayV2HTTPResponse {
	var res *events.APIGatewayV2HTTPResponse = nil
	req, err := newApiRequest(origReq)

	if err == nil {
		res, err = h.handleApiRequest(req)
	}

	if err != nil {
		res = h.errorResponse(err)
	}
	logApiResponse(origReq, res, err)
	return res
}

func (h *Handler) addResponseBody(
	res *events.APIGatewayV2HTTPResponse, body string,
) {
	httpStatus := res.StatusCode
	title := fmt.Sprintf("%d %s", httpStatus, http.StatusText(httpStatus))
	params := &ResponseTemplateParams{title, h.SiteTitle, body}
	builder := &strings.Builder{}

	if err := h.responseTemplate.Execute(builder, params); err != nil {
		// This should never happen, but if it does, fall back to plain text.
		log.Printf("ERROR adding HTML response body: %s: %+v", err, params)
		res.Headers["content-type"] = "text/plain; charset=utf-8"
		res.Body = fmt.Sprintf("%s - %s\n\n%s\n", title, h.SiteTitle, body)
	} else {
		res.Headers["content-type"] = "text/html; charset=utf-8"
		res.Body = builder.String()
	}
}

func (h *Handler) errorResponse(err error) *events.APIGatewayV2HTTPResponse {
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

func (h *Handler) handleApiRequest(
	req *apiRequest,
) (*events.APIGatewayV2HTTPResponse, error) {
	res := &events.APIGatewayV2HTTPResponse{Headers: map[string]string{}}
	res.Headers["content-type"] = "text/plain; charset=utf-8"

	if op, err := parseApiRequest(req); err != nil {
		return h.respondToParseError(res, err)
	} else if result, err := h.performOperation(op); err != nil {
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

func (h *Handler) respondToParseError(
	response *events.APIGatewayV2HTTPResponse, err error,
) (*events.APIGatewayV2HTTPResponse, error) {
	// Treat email parse errors differently for the Subscribe operation, since
	// it may be due to a user typo. In all other cases, the assumption is that
	// it's a bad machine generated request.
	if !errors.Is(err, &ParseError{Type: SubscribeOp}) {
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

func (h *Handler) performOperation(
	op *eventOperation,
) (ops.OperationResult, error) {
	result := ops.Invalid
	var err error = nil

	switch op.Type {
	case SubscribeOp:
		result, err = h.Agent.Subscribe(op.Email)
	case VerifyOp:
		result, err = h.Agent.Verify(op.Email, op.Uid)
	case UnsubscribeOp:
		result, err = h.Agent.Unsubscribe(op.Email, op.Uid)
	default:
		err = fmt.Errorf("can't handle operation type: %s", op.Type)
	}

	if opErr, ok := err.(*ops.OperationErrorExternal); ok {
		err = &errorWithStatus{http.StatusBadGateway, opErr.Error()}
	}
	return result, err
}

func newMailtoEvent(ses *events.SimpleEmailService) *mailtoEvent {
	headers := ses.Mail.CommonHeaders
	receipt := &ses.Receipt

	return &mailtoEvent{
		From:         headers.From,
		To:           headers.To,
		Subject:      headers.Subject,
		MessageId:    ses.Mail.MessageID,
		SpfVerdict:   receipt.SPFVerdict.Status,
		DkimVerdict:  receipt.DKIMVerdict.Status,
		SpamVerdict:  receipt.SpamVerdict.Status,
		VirusVerdict: receipt.VirusVerdict.Status,
		DmarcVerdict: receipt.DMARCVerdict.Status,
		DmarcPolicy:  receipt.DMARCPolicy,
	}
}

// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-example-functions.html
// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-contents.html
// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-examples.html
func (h *Handler) handleMailtoEvent(ev *mailtoEvent) error {
	prefix := "unsubscribe message " + ev.MessageId + ": "

	if bounced, err := h.bounceIfDmarcFails(ev); err != nil {
		return fmt.Errorf("%sDMARC bounce fail: %s: %s", prefix, meta(ev), err)
	} else if bounced {
		log.Printf("%sDMARC bounce: %s", prefix, meta(ev))
	} else if isSpam(ev) {
		log.Printf("%smarked as spam, ignored: %s", prefix, meta(ev))
	} else if op, err := parseMailtoEvent(ev, h.UnsubscribeAddr); err != nil {
		log.Printf("%sfailed to parse, ignoring: %s: %s", prefix, meta(ev), err)
	} else if result, err := h.Agent.Unsubscribe(op.Email, op.Uid); err != nil {
		return fmt.Errorf("%serror: %s: %s", prefix, op.Email, err)
	} else if result != ops.Unsubscribed {
		log.Printf("%sfailed: %s: %s", prefix, op.Email, result)
	} else {
		log.Printf("%ssuccess: %s", prefix, op.Email)
	}
	return nil
}

func meta(ev *mailtoEvent) string {
	return fmt.Sprintf(
		"[From:\"%s\" To:\"%s\" Subject:\"%s\"]",
		strings.Join(ev.From, ","),
		strings.Join(ev.To, ","),
		ev.Subject,
	)
}

func (h *Handler) bounceIfDmarcFails(ev *mailtoEvent) (bool, error) {
	return false, nil
}

func isSpam(ev *mailtoEvent) bool {
	return false
}
