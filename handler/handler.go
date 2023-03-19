package handler

import (
	"context"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

const defaultResponseLocation = "https://github.com/mbland/ses-subscription-verifier"

type LambdaHandler struct {
	Db Database
}

func (*LambdaHandler) HandleRequest(
	ctx context.Context, event events.APIGatewayV2HTTPRequest,
) (events.APIGatewayV2HTTPResponse, error) {
	response := events.APIGatewayV2HTTPResponse{Headers: make(map[string]string)}
	response.StatusCode = http.StatusSeeOther
	response.Headers["Location"] = defaultResponseLocation

	return response, nil
}
