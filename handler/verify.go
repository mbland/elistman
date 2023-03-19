package handler

import "context"

type VerifyHandler interface {
	HandleRequest(ctx context.Context)
}

type ProdVerifyHandler struct {
	Db     Database
	Mailer Mailer
}

func (h ProdVerifyHandler) HandleRequest(ctx context.Context) {
}
