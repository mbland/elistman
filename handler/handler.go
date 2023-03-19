package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

const defaultResponseLocation = "https://github.com/mbland/ses-subscription-verifier"

type LambdaHandler struct {
	SubscribeHandler SubscribeHandler
	VerifyHandler    VerifyHandler
}

func (h LambdaHandler) HandleRequest(
	ctx context.Context, request events.APIGatewayV2HTTPRequest,
) (events.APIGatewayV2HTTPResponse, error) {
	response := events.APIGatewayV2HTTPResponse{Headers: make(map[string]string)}
	endpoint := strings.TrimPrefix(request.RawPath, "/email")
	response.StatusCode = http.StatusSeeOther
	response.Headers["Location"] = defaultResponseLocation

	if endpoint == "/subscribe" {
		h.SubscribeHandler.HandleRequest(ctx)

	} else if endpoint == "/verify" {
		h.VerifyHandler.HandleRequest(ctx)

	} else {
		response.StatusCode = http.StatusNotFound
	}
	return response, nil
}
