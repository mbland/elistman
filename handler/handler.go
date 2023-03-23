package handler

import (
	"fmt"
	"net/http"

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
		return nil, h.handleMailtoEvent(event.MailtoEvent)
	}
	return nil, fmt.Errorf("unexpected event: %+v", event)
}

func (h LambdaHandler) handleApiRequest(
	request events.APIGatewayV2HTTPRequest,
) (events.APIGatewayV2HTTPResponse, error) {
	response := events.APIGatewayV2HTTPResponse{Headers: make(map[string]string)}
	response.StatusCode = http.StatusSeeOther

	if request.RawPath == "/subscribe" {
		h.SubscribeHandler.HandleRequest()
		response.Headers["Location"] = defaultResponseLocation

	} else if request.RawPath == "/verify" {
		h.VerifyHandler.HandleRequest()
		response.Headers["Location"] = defaultResponseLocation

	} else {
		response.StatusCode = http.StatusNotFound
	}
	return response, nil
}

func (h LambdaHandler) handleMailtoEvent(event events.SimpleEmailEvent) error {
	return nil
}
