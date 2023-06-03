package handler

import (
	"context"
	"log"

	"github.com/mbland/elistman/agent"
	"github.com/mbland/elistman/email"
)

type cliHandler struct {
	Agent agent.SubscriptionAgent
	Log   *log.Logger
}

func (h *cliHandler) HandleEvent(
	ctx context.Context, e *email.SendEvent,
) (res *email.SendResponse) {
	res = &email.SendResponse{}
	var err error

	if res.NumSent, err = h.Agent.Send(ctx, &e.Message); err != nil {
		res.Details = err.Error()
	} else {
		res.Success = true
	}

	const logFmt = "send: subject: \"%s\"; success: %t; num sent: %d"
	h.Log.Printf(logFmt, e.Message.Subject, res.Success, res.NumSent)
	return
}
