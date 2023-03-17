package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/mbland/ses-subscription-verifier/handler"
)

func main() {
	h := handler.LambdaHandler{}
	lambda.Start(h.HandleRequest)
}
