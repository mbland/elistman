package handler

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/ops"
)

type RedirectMap map[ops.OperationResult]string

type Handler struct {
	UnsubscribeAddr string
	Agent           ops.SubscriptionAgent
	Redirects       RedirectMap
}

func NewHandler(
	emailDomain string,
	agent ops.SubscriptionAgent,
	paths RedirectPaths,
) *Handler {
	fullUrl := func(path string) string {
		return "https://" + emailDomain + "/" + path
	}
	return &Handler{
		"unsubscribe@" + emailDomain,
		agent,
		RedirectMap{
			ops.Invalid:           fullUrl(paths.Invalid),
			ops.AlreadySubscribed: fullUrl(paths.AlreadySubscribed),
			ops.VerifyLinkSent:    fullUrl(paths.VerifyLinkSent),
			ops.Subscribed:        fullUrl(paths.Subscribed),
			ops.NotSubscribed:     fullUrl(paths.NotSubscribed),
			ops.Unsubscribed:      fullUrl(paths.Unsubscribed),
		},
	}
}

func (h *Handler) HandleEvent(event *Event) (any, error) {
	switch event.Type {
	case ApiRequest:
		if req, err := newApiRequest(&event.ApiRequest); err != nil {
			return nil, err
		} else {
			return h.handleApiRequest(req)
		}
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
	return nil, nil
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
	res := &events.APIGatewayV2HTTPResponse{Headers: make(map[string]string)}
	res.Headers["content-type"] = "text/plain; charset=utf-8"

	if op, err := parseApiRequest(req); err != nil {
		return h.respondToParseError(res, err)
	} else if result, err := h.performOperation(op, err); err != nil {
		res.StatusCode = http.StatusInternalServerError
		return res, err
	} else if op.OneClick {
		res.StatusCode = http.StatusOK
	} else if redirect, ok := h.Redirects[result]; !ok {
		res.StatusCode = http.StatusInternalServerError
		return res, fmt.Errorf("no redirect for op result: %s", result)
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
		response.Body = fmt.Sprintf("%s\n", err)
	} else if redirect, ok := h.Redirects[ops.Invalid]; !ok {
		response.StatusCode = http.StatusInternalServerError
		return response, fmt.Errorf("no redirect for invalid operation")
	} else {
		response.StatusCode = http.StatusSeeOther
		response.Headers["location"] = redirect
	}
	return response, nil
}

func (h *Handler) performOperation(
	op *eventOperation, err error,
) (ops.OperationResult, error) {
	switch op.Type {
	case SubscribeOp:
		return h.Agent.Subscribe(op.Email)
	case VerifyOp:
		return h.Agent.Verify(op.Email, op.Uid)
	case UnsubscribeOp:
		return h.Agent.Unsubscribe(op.Email, op.Uid)
	}
	return ops.Invalid, fmt.Errorf("can't handle operation type: %s", op.Type)
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
