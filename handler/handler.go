package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/ops"
)

const defaultResponseLocation = "https://github.com/mbland/elistman"

type LambdaHandler struct {
	Agent ops.SubscriptionAgent
}

func (h LambdaHandler) HandleEvent(event Event) (any, error) {
	switch event.Type {
	case ApiRequest:
		return h.handleApiRequest(event.ApiRequest)
	case MailtoEvent:
		return nil, h.handleMailtoEvent(
			event.MailtoEvent, "unsubscribe@"+os.Getenv("EMAIL_DOMAIN_NAME"),
		)
	default:
		return nil, nil
	}
}

func (h LambdaHandler) handleApiRequest(
	request events.APIGatewayV2HTTPRequest,
) (events.APIGatewayV2HTTPResponse, error) {
	response := events.APIGatewayV2HTTPResponse{Headers: make(map[string]string)}
	response.Headers["Content-Type"] = "text/plain; charset=utf-8"
	op, err := parseApiEvent(request.RawPath, request.PathParameters)

	if err != nil {
		h.prepareParseErrorResponse(request.RawPath, &response, err)
		return response, nil
	}

	ok := false

	switch op.Type {
	case SubscribeOp:
		ok, err = h.Agent.Subscribe(op.Email)
	case VerifyOp:
		ok, err = h.Agent.Verify(op.Email, op.Uid)
	case UnsubscribeOp:
		ok, err = h.Agent.Unsubscribe(op.Email, op.Uid)
	default:
		response.StatusCode = http.StatusNotFound
	}

	if err != nil {
		response.StatusCode = http.StatusInternalServerError
		return response, err
	} else if ok {
		log.Printf("TODO: Redirect to success page")
	} else {
		log.Printf("TODO: Redirect to error page")
	}
	response.StatusCode = http.StatusSeeOther
	response.Headers["Location"] = defaultResponseLocation
	return response, nil
}

func (h LambdaHandler) prepareParseErrorResponse(
	endpoint string, response *events.APIGatewayV2HTTPResponse, err error,
) {
	// Treat email parse errors differently for the Subscribe operation, since
	// it may be due to a user typo. In all other cases, the assumption is that
	// it's a bad machine generated request.
	if strings.Contains(err.Error(), "invalid email address") &&
		strings.HasPrefix(endpoint, SubcribePrefix) {
		response.StatusCode = http.StatusSeeOther
		response.Headers["Location"] = defaultResponseLocation
	} else {
		response.StatusCode = http.StatusBadRequest
		response.Body = fmt.Sprintf("%s\n", err)
	}
}

func (h LambdaHandler) handleMailtoEvent(
	event events.SimpleEmailEvent, unsubscribeRecipient string,
) error {
	ses := event.Records[0].SES
	headers := ses.Mail.CommonHeaders

	if isSpam(ses.Receipt) {
		return nil
	} else if op, err := parseMailtoEvent(
		headers.From, headers.To, unsubscribeRecipient, headers.Subject,
	); err != nil {
		log.Printf("error parsing mailto event, ignoring: %s", err)
	} else if ok, err := h.Agent.Unsubscribe(op.Email, op.Uid); err != nil {
		return fmt.Errorf("error while unsubscribing %s: %s", op.Email, err)
	} else if ok {
		log.Printf("unsubscribed: %s", op.Email)
	}
	return nil
}

// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-example-functions.html
// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-contents.html
func isSpam(receipt events.SimpleEmailReceipt) bool {
	return false
}
