package handler

import (
	"context"
	"errors"

	"github.com/aws/aws-lambda-go/events"
)

func Handler(
	ctx context.Context, event events.APIGatewayV2HTTPRequest,
) (events.APIGatewayV2HTTPResponse, error) {
	response := events.APIGatewayV2HTTPResponse{Headers: make(map[string]string)}

	return response, errors.New("Not yet implemented")
}
