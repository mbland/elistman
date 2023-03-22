package ops

import (
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
)

type SubscribeHandler interface {
	HandleRequest()
}

type ProdSubscribeHandler struct {
	Db        db.Database
	Validator email.AddressValidator
	Mailer    email.Mailer
}

func (h ProdSubscribeHandler) HandleRequest() {
}
