package handler

import (
	"context"
	"fmt"
	"log"

	"github.com/mbland/elistman/agent"
	"github.com/mbland/elistman/email"
)

type Handler struct {
	api    *apiHandler
	mailto *mailtoHandler
	sns    *snsHandler
	cli    *cliHandler
}

func NewHandler(
	emailDomain string,
	siteTitle string,
	agent agent.SubscriptionAgent,
	paths RedirectPaths,
	responseTemplate string,
	unsubscribeUserName string,
	bouncer email.Bouncer,
	logger *log.Logger,
) (*Handler, error) {
	api, err := newApiHandler(
		emailDomain, siteTitle, agent, paths, responseTemplate, logger,
	)

	if err != nil {
		return nil, err
	}

	unsubAddr := unsubscribeUserName + "@" + emailDomain
	return &Handler{
		api,
		&mailtoHandler{emailDomain, unsubAddr, agent, bouncer, logger},
		&snsHandler{agent, logger},
		&cliHandler{agent, logger},
	}, nil
}

const ResponseTemplate = `<!DOCTYPE html>
<html lang="en-us">
  <head>
	<meta charset="utf-8"/>
	<meta name="viewport" content="width=device-width, initial-scale=1"/>
	<title>{{.Title}} - {{.SiteTitle}}</title>
  </head>
  <body>
    <h1>{{.Title}}</h1>
    {{.Body}}
  </body>
</html>`

func (h *Handler) HandleEvent(
	ctx context.Context, event *Event,
) (result any, err error) {
	switch event.Type {
	case ApiRequest:
		result = h.api.HandleEvent(ctx, event.ApiRequest)
	case MailtoEvent:
		result = h.mailto.HandleEvent(ctx, event.MailtoEvent)
	case SnsEvent:
		h.sns.HandleEvent(ctx, event.SnsEvent)
	case CommandLineEvent:
		result = h.cli.HandleEvent(ctx, event.CommandLineEvent)
	case UnknownEvent:
		// An unknown event is one that Event.UnmarshalJSON knows nothing about.
		err = fmt.Errorf("unknown event: %s", string(event.Unknown))
	default:
		// An unexpected event is one that Event.UnmarshalJSON can parse,
		// but this function knows nothing about.
		const errFmt = "unexpected event type: %s: %+v"
		err = fmt.Errorf(errFmt, event.Type, event)
	}
	return
}
