package handler

import (
	"context"
	"fmt"
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
) (res any, err error) {
	switch e.EListManCommand {
	case events.CommandLineSendEvent:
		res = h.HandleSendEvent(ctx, e.Send)
	default:
		err = fmt.Errorf("unknown EListMan command: %s", e.EListManCommand)
	}
	return
}

func (h *cliHandler) HandleSendEvent(
	ctx context.Context, e *events.SendEvent,
) (res *events.SendResponse) {
	res = &events.SendResponse{}
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
