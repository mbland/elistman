package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

const defaultResponseLocation = "https://github.com/mbland/ses-subscription-verifier"

type LambdaHandler struct {
	SubscribeHandler SubscribeHandler
	VerifyHandler    VerifyHandler
}

func getEndpoint(request events.APIGatewayV2HTTPRequest) string {
	if request.RouteKey == "" {
		return request.RawPath
	}
	route_prefix := fmt.Sprintf("/%s", request.RouteKey)
	return strings.TrimPrefix(request.RawPath, route_prefix)
}

func (h LambdaHandler) HandleRequest(
	request events.APIGatewayV2HTTPRequest,
) (events.APIGatewayV2HTTPResponse, error) {
	endpoint := getEndpoint(request)
	response := events.APIGatewayV2HTTPResponse{Headers: make(map[string]string)}
	response.StatusCode = http.StatusSeeOther
	response.Headers["Location"] = defaultResponseLocation

	if endpoint == "/subscribe" {
		h.SubscribeHandler.HandleRequest()

	} else if endpoint == "/verify" {
		h.VerifyHandler.HandleRequest()

	} else {
		response.StatusCode = http.StatusNotFound
	}
	return response, nil
}
