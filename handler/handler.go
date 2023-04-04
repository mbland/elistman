package handler

import (
	"fmt"

	"github.com/mbland/elistman/ops"
)

type Handler struct {
	api    *apiHandler
	mailto *mailtoHandler
}

func NewHandler(
	emailDomain string,
	siteTitle string,
	agent ops.SubscriptionAgent,
	paths RedirectPaths,
	responseTemplate string,
) (*Handler, error) {
	api, err := newApiHandler(
		emailDomain, siteTitle, agent, paths, responseTemplate,
	)

	if err != nil {
		return nil, err
	}
	return &Handler{
		api, &mailtoHandler{"unsubscribe@" + emailDomain, agent},
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

func (h *Handler) HandleEvent(event *Event) (any, error) {
	switch event.Type {
	case ApiRequest:
		return h.api.HandleEvent(&event.ApiRequest), nil
	case MailtoEvent:
		h.mailto.HandleEvent(&event.MailtoEvent)
	}
	return nil, fmt.Errorf("unexpected event type: %s: %+v", event.Type, event)
}
