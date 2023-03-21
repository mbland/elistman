package handler

type VerifyHandler interface {
	HandleRequest()
}

type ProdVerifyHandler struct {
	Db     Database
	Mailer Mailer
}

func (h ProdVerifyHandler) HandleRequest() {
}
