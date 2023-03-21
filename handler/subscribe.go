package handler

type SubscribeHandler interface {
	HandleRequest()
}

type ProdSubscribeHandler struct {
	Db        Database
	Validator AddressValidator
	Mailer    Mailer
}

func (h ProdSubscribeHandler) HandleRequest() {
}
