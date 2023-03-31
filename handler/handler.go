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
		return h.handleApiRequest(newApiRequest(&event.ApiRequest))
	case MailtoEvent:
		if ev, err := newMailtoEvent(&event.MailtoEvent); err != nil {
			return nil, err
		} else {
			return nil, h.handleMailtoEvent(ev)
		}
	}
	return nil, nil
}

func newApiRequest(req *events.APIGatewayV2HTTPRequest) *apiRequest {
	params := map[string]string{}

	for k, v := range req.PathParameters {
		params[k] = v
	}
	return &apiRequest{
		req.RawPath,
		req.RequestContext.HTTP.Method,
		req.Headers["Content-Type"],
		params,
		req.Body,
	}
}

func (h *Handler) handleApiRequest(
	req *apiRequest,
) (*events.APIGatewayV2HTTPResponse, error) {
	res := &events.APIGatewayV2HTTPResponse{Headers: make(map[string]string)}
	res.Headers["Content-Type"] = "text/plain; charset=utf-8"

	if op, err := parseApiRequest(req); err != nil {
		return h.respondToParseError(res, err)
	} else if result, err := h.performOperation(op, err); err != nil {
		res.StatusCode = http.StatusInternalServerError
		return res, err
	} else if isOneClickUnsubscribeRequest(op, req) {
		res.StatusCode = http.StatusOK
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

func isOneClickUnsubscribeRequest(op *eventOperation, req *apiRequest) bool {
	return op.Type == UnsubscribeOp &&
		req.Method == http.MethodPost &&
		req.Params["List-Unsubscribe"] == "One-Click"
}

func newMailtoEvent(e *events.SimpleEmailEvent) (*mailtoEvent, error) {
	if len(e.Records) != 1 {
		return nil, fmt.Errorf(
			"expected one SES event Record, got %d", len(e.Records),
		)
	}

	ses := e.Records[0].SES
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
	prefix := "message " + ev.MessageId + ": "

	if bounced, err := h.bounceIfDmarcFails(ev); err != nil {
		return fmt.Errorf("%serror while bouncing: %s", prefix, err)
	} else if bounced {
		log.Printf("%sbounced", prefix)
	} else if isSpam(ev) {
		log.Printf("%sspam, ignoring", prefix)
	} else if op, err := parseMailtoEvent(ev, h.UnsubscribeAddr); err != nil {
		log.Printf("%sparse error, ignoring: %s", prefix, err)
	} else if result, err := h.Agent.Unsubscribe(op.Email, op.Uid); err != nil {
		return fmt.Errorf("%serror unsubscribing %s: %s", prefix, op.Email, err)
	} else if result != ops.Unsubscribed {
		log.Printf("%snot unsubscribed: %s: %s", prefix, op.Email, result)
	} else {
		log.Printf("%ssuccessfully unsubscribed: %s", prefix, op.Email)
	}
	return nil
}

func (h *Handler) bounceIfDmarcFails(ev *mailtoEvent) (bool, error) {
	return false, nil
}

func isSpam(ev *mailtoEvent) bool {
	return false
}
