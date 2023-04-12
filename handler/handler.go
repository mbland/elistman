package handler

import (
	"fmt"
	"log"

	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/ops"
)

type Handler struct {
	api    *apiHandler
	mailto *mailtoHandler
	sns    *snsHandler
}

func NewHandler(
	emailDomain string,
	siteTitle string,
	agent ops.SubscriptionAgent,
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

func (h *Handler) HandleEvent(event *Event) (result any, err error) {
	switch event.Type {
	case ApiRequest:
		result = h.api.HandleEvent(event.ApiRequest)
	case MailtoEvent:
		result = h.mailto.HandleEvent(event.MailtoEvent)
	case SnsEvent:
		h.sns.HandleEvent(event.SnsEvent)
	default:
		err = fmt.Errorf("unexpected event type: %s: %+v", event.Type, event)
	}
	return
}
