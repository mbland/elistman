package ops

import (
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
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
