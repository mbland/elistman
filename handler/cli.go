package handler

import (
	"context"
	"log"

	"github.com/mbland/elistman/agent"
	"github.com/mbland/elistman/events"
)

type cliHandler struct {
	Agent agent.SubscriptionAgent
	Log   *log.Logger
}

func (h *cliHandler) HandleEvent(
	ctx context.Context, e *events.CommandLineEvent,
) (res *events.SendResponse) {
	sendEvent := e.Send
	res = &events.SendResponse{}
	var err error

	if res.NumSent, err = h.Agent.Send(ctx, &sendEvent.Message); err != nil {
		res.Details = err.Error()
	} else {
		res.Success = true
	}

	const logFmt = "send: subject: \"%s\"; success: %t; num sent: %d"
	h.Log.Printf(logFmt, sendEvent.Message.Subject, res.Success, res.NumSent)
	return
}
