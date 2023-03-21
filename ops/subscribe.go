package ops

import (
	"github.com/mbland/ses-subscription-verifier/db"
	"github.com/mbland/ses-subscription-verifier/email"
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
