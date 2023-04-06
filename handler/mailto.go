package handler

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/ops"
)

type mailtoHandler struct {
	EmailDomain     string
	Agent           ops.SubscriptionAgent
	Bouncer         email.Bouncer
	Log             *log.Logger
	unsubscribeAddr string
}

func newMailtoHandler(
	emailDomain string,
	agent ops.SubscriptionAgent,
	bouncer email.Bouncer,
	log *log.Logger,
) *mailtoHandler {
	return &mailtoHandler{
		emailDomain, agent, bouncer, log, "unsubscribe@" + emailDomain,
	}
}

func (h *mailtoHandler) HandleEvent(e *events.SimpleEmailEvent) {
	// If I understand the contract correctly, there should only ever be one
	// valid Record per event. However, we have the technology to deal
	// gracefully with the unexpected.
	errs := make([]error, len(e.Records))

	for i, record := range e.Records {
		errs[i] = h.handleMailtoEvent(newMailtoEvent(&record.SES))
	}
	h.Log.Printf("ERROR from mailto requests: %s", errors.Join(errs...))
}

func newMailtoEvent(ses *events.SimpleEmailService) *mailtoEvent {
	headers := ses.Mail.CommonHeaders
	receipt := &ses.Receipt

	// Notice that according to:
	// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-example-functions.html
	//
	// all of the below verdicts and the DMARCPolicy should be all uppercase.
	//
	// However, according to:
	// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-contents.html
	//
	// The DMARCPolicy should be all lowercase. As a defensive measure, we
	// explicitly uppercase them all.
	return &mailtoEvent{
		From:         headers.From,
		To:           headers.To,
		Subject:      headers.Subject,
		MessageId:    ses.Mail.MessageID,
		Timestamp:    receipt.Timestamp,
		Recipients:   receipt.Recipients,
		SpfVerdict:   strings.ToUpper(receipt.SPFVerdict.Status),
		DkimVerdict:  strings.ToUpper(receipt.DKIMVerdict.Status),
		SpamVerdict:  strings.ToUpper(receipt.SpamVerdict.Status),
		VirusVerdict: strings.ToUpper(receipt.VirusVerdict.Status),
		DmarcVerdict: strings.ToUpper(receipt.DMARCVerdict.Status),
		DmarcPolicy:  strings.ToUpper(receipt.DMARCPolicy),
	}
}

// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-example-functions.html
// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-contents.html
// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-examples.html
func (h *mailtoHandler) handleMailtoEvent(ev *mailtoEvent) error {
	prefix := "unsubscribe message " + ev.MessageId + ": "

	if bounceMessageId, err := h.bounceIfDmarcFails(ev); err != nil {
		return fmt.Errorf("%sDMARC bounce fail %s: %s", prefix, meta(ev), err)
	} else if bounceMessageId != "" {
		const errFmt = "%sDMARC bounced %s with bounce message ID: %s"
		h.Log.Printf(errFmt, prefix, meta(ev), bounceMessageId)
	} else if isSpam(ev) {
		h.Log.Printf("%smarked as spam, ignored: %s", prefix, meta(ev))
	} else if op, err := parseMailtoEvent(ev, h.unsubscribeAddr); err != nil {
		const errFmt = "%sfailed to parse, ignoring: %s: %s"
		h.Log.Printf(errFmt, prefix, meta(ev), err)
	} else if result, err := h.Agent.Unsubscribe(op.Email, op.Uid); err != nil {
		return fmt.Errorf("%serror: %s: %s", prefix, op.Email, err)
	} else if result != ops.Unsubscribed {
		h.Log.Printf("%sfailed: %s: %s", prefix, op.Email, result)
	} else {
		h.Log.Printf("%ssuccess: %s", prefix, op.Email)
	}
	return nil
}

func meta(ev *mailtoEvent) string {
	return fmt.Sprintf(
		"[From:\"%s\" To:\"%s\" Subject:\"%s\"]",
		strings.Join(ev.From, ","),
		strings.Join(ev.To, ","),
		ev.Subject,
	)
}

func (h *mailtoHandler) bounceIfDmarcFails(
	ev *mailtoEvent,
) (bounceMessageId string, err error) {
	if ev.DmarcVerdict == "FAIL" && ev.DmarcPolicy == "REJECT" {
		bounceMessageId, err = h.Bouncer.Bounce(
			h.EmailDomain, ev.Recipients, ev.Timestamp,
		)
	}
	return
}

func isSpam(ev *mailtoEvent) bool {
	return false
}
