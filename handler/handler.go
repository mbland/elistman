package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/ops"
)

const defaultResponseLocation = "https://github.com/mbland/elistman"

type LambdaHandler struct {
	SubscribeHandler ops.SubscribeHandler
	VerifyHandler    ops.VerifyHandler
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
	response.StatusCode = http.StatusSeeOther
	op, err := parseApiRequestOperation(request.RawPath, request.PathParameters)

	if err != nil {
		response.StatusCode = http.StatusBadRequest
		response.Body = err.Error()
		return response, nil
	}

	switch op.Type {
	case SubscribeOp:
		h.SubscribeHandler.HandleRequest()
		response.Headers["Location"] = defaultResponseLocation
	case VerifyOp:
		h.VerifyHandler.HandleRequest()
		response.Headers["Location"] = defaultResponseLocation
	case UnsubscribeOp:
	default:
		response.StatusCode = http.StatusNotFound
	}
	return response, nil
}

func (h LambdaHandler) handleMailtoEvent(
	event events.SimpleEmailEvent, unsubscribe_recipient string,
) error {
	headers := event.Records[0].SES.Mail.CommonHeaders
	op, err := parseMailtoEventOperation(
		headers.From, headers.To, unsubscribe_recipient, headers.Subject,
	)

	if err != nil {
		log.Printf("error parsing mailto event, ignoring: %s", err)
		return nil
	}
	fmt.Print(op) // remove this when implemented
	return nil
}
