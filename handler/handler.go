package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"

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
	paths RedirectPaths,
	agent ops.SubscriptionAgent,
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
		return h.handleApiRequest(&event.ApiRequest)
	case MailtoEvent:
		return nil, h.handleMailtoEvent(&event.MailtoEvent)
	}
	return nil, nil
}

func (h *Handler) handleApiRequest(
	req *events.APIGatewayV2HTTPRequest,
) (*events.APIGatewayV2HTTPResponse, error) {
	res := &events.APIGatewayV2HTTPResponse{Headers: make(map[string]string)}
	res.Headers["Content-Type"] = "text/plain; charset=utf-8"
	endpoint, params := req.RawPath, req.PathParameters

	if op, err := parseApiEvent(endpoint, params); err != nil {
		return h.respondToParseError(res, err)
	} else if result, err := h.performOperation(op, err); err != nil {
		res.StatusCode = http.StatusInternalServerError
		return res, err
	} else if redirect, ok := h.Redirects[result]; !ok {
		res.StatusCode = http.StatusInternalServerError
		return res, fmt.Errorf("no redirect for op result: %s", result)
	} else {
		res.StatusCode = http.StatusSeeOther
		res.Headers["Location"] = redirect
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
		response.Headers["Location"] = redirect
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

// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-example-functions.html
// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-contents.html
// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-examples.html
func (h *Handler) handleMailtoEvent(event *events.SimpleEmailEvent) error {
	ses := event.Records[0].SES
	headers := ses.Mail.CommonHeaders

	if isSpam(ses.Receipt) {
		log.Printf("received spam, ignoring")
	} else if op, err := parseMailtoEvent(
		headers.From, headers.To, h.UnsubscribeAddr, headers.Subject,
	); err != nil {
		log.Printf("error parsing mailto event, ignoring: %s", err)
	} else if result, err := h.Agent.Unsubscribe(op.Email, op.Uid); err != nil {
		return fmt.Errorf("error while unsubscribing %s: %s", op.Email, err)
	} else if result == ops.Unsubscribed {
		log.Printf("unsubscribed: %s", op.Email)
	}
	return nil
}

func isSpam(receipt events.SimpleEmailReceipt) bool {
	return false
}
