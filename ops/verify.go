package ops

import (
	"github.com/mbland/ses-subscription-verifier/db"
	"github.com/mbland/ses-subscription-verifier/email"
)

type VerifyHandler interface {
	HandleRequest()
}

type ProdVerifyHandler struct {
	Db     db.Database
	Mailer email.Mailer
}

func (h ProdVerifyHandler) HandleRequest() {
}
