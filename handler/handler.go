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
	response.Headers["Content-Type"] = "text/plain; charset=utf-8"
	op, err := parseApiRequestOperation(request.RawPath, request.PathParameters)

	if err != nil {
		h.prepareParseErrorResponse(request.RawPath, &response, err)
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
	response.StatusCode = http.StatusSeeOther
	return response, nil
}

func (h LambdaHandler) prepareParseErrorResponse(
	endpoint string, response *events.APIGatewayV2HTTPResponse, err error,
) {
	// Treat email parse errors differently for the Subscribe operation, since
	// it may be due to a user typo. In all other cases, the assumption is that
	// it's a bad machine generated request.
	if strings.Contains(err.Error(), "failed to parse email") &&
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
	headers := event.Records[0].SES.Mail.CommonHeaders
	op, err := parseMailtoEventOperation(
		headers.From, headers.To, unsubscribeRecipient, headers.Subject,
	)

	if err != nil {
		log.Printf("error parsing mailto event, ignoring: %s", err)
		return nil
	}
	fmt.Print(op) // remove this when implemented
	return nil
}
