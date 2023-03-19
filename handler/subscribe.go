package handler

import "context"

type SubscribeHandler interface {
	HandleRequest(ctx context.Context)
}

type ProdSubscribeHandler struct {
	Db        Database
	Validator AddressValidator
	Mailer    Mailer
}

func (h ProdSubscribeHandler) HandleRequest(ctx context.Context) {
}
