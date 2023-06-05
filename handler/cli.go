package handler

import (
	"context"
	"fmt"
	"log"
	"strings"

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
	case events.CommandLineImportEvent:
		res = h.HandleImportEvent(ctx, e.Import)
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

func (h *cliHandler) HandleImportEvent(
	ctx context.Context, e *events.ImportEvent,
) (response *events.ImportResponse) {
	failures := make([]string, 0, len(e.Addresses))
	imported := make([]string, 0, len(e.Addresses))

	for _, addr := range e.Addresses {
		if err := h.Agent.Import(ctx, addr); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", addr, err))
		} else {
			imported = append(imported, addr)
		}
	}

	response = &events.ImportResponse{NumImported: len(imported)}

	if len(imported) != 0 {
		importedList := strings.Join(imported, ", ")
		h.Log.Printf("imported %d: %s", len(imported), importedList)
	}
	if len(failures) != 0 {
		failureList := strings.Join(failures, "\n  ")
		h.Log.Printf("failed to import %d:\n  %s", len(failures), failureList)
		response.Failures = failures
	}
	return
}
