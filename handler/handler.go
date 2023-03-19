package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

const defaultResponseLocation = "https://github.com/mbland/ses-subscription-verifier"

type LambdaHandler struct {
	Db        Database
	Validator AddressValidator
	Mailer    Mailer
}

func (h LambdaHandler) HandleRequest(
	ctx context.Context, request events.APIGatewayV2HTTPRequest,
) (events.APIGatewayV2HTTPResponse, error) {
	response := events.APIGatewayV2HTTPResponse{Headers: make(map[string]string)}
	endpoint := strings.TrimPrefix(request.RawPath, "/email")
	response.StatusCode = http.StatusSeeOther
	response.Headers["Location"] = defaultResponseLocation

	if endpoint == "/subscribe" {
		h.HandleSubscribe(ctx)

	} else if endpoint == "/verify" {
		h.HandleVerify(ctx)

	} else {
		response.StatusCode = http.StatusNotFound
	}
	return response, nil
}

func (h LambdaHandler) HandleSubscribe(ctx context.Context) {
}

func (h LambdaHandler) HandleVerify(ctx context.Context) {
}
