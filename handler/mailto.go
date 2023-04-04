package handler

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/ops"
)

type mailtoHandler struct {
	UnsubscribeAddr string
	Agent           ops.SubscriptionAgent
}

func (h *mailtoHandler) HandleEvent(e *events.SimpleEmailEvent) {
	// If I understand the contract correctly, there should only ever be one
	// valid Record per event. However, we have the technology to deal
	// gracefully with the unexpected.
	errs := make([]error, len(e.Records))

	for i, record := range e.Records {
		errs[i] = h.handleMailtoEvent(newMailtoEvent(&record.SES))
	}
	log.Printf("ERROR from mailto requests: %s", errors.Join(errs...))
}

func newMailtoEvent(ses *events.SimpleEmailService) *mailtoEvent {
	headers := ses.Mail.CommonHeaders
	receipt := &ses.Receipt

	return &mailtoEvent{
		From:         headers.From,
		To:           headers.To,
		Subject:      headers.Subject,
		MessageId:    ses.Mail.MessageID,
		SpfVerdict:   receipt.SPFVerdict.Status,
		DkimVerdict:  receipt.DKIMVerdict.Status,
		SpamVerdict:  receipt.SpamVerdict.Status,
		VirusVerdict: receipt.VirusVerdict.Status,
		DmarcVerdict: receipt.DMARCVerdict.Status,
		DmarcPolicy:  receipt.DMARCPolicy,
	}
}

// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-example-functions.html
// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-contents.html
// - https://docs.aws.amazon.com/ses/latest/dg/receiving-email-notifications-examples.html
func (h *mailtoHandler) handleMailtoEvent(ev *mailtoEvent) error {
	prefix := "unsubscribe message " + ev.MessageId + ": "

	if bounced, err := h.bounceIfDmarcFails(ev); err != nil {
		return fmt.Errorf("%sDMARC bounce fail: %s: %s", prefix, meta(ev), err)
	} else if bounced {
		log.Printf("%sDMARC bounce: %s", prefix, meta(ev))
	} else if isSpam(ev) {
		log.Printf("%smarked as spam, ignored: %s", prefix, meta(ev))
	} else if op, err := parseMailtoEvent(ev, h.UnsubscribeAddr); err != nil {
		log.Printf("%sfailed to parse, ignoring: %s: %s", prefix, meta(ev), err)
	} else if result, err := h.Agent.Unsubscribe(op.Email, op.Uid); err != nil {
		return fmt.Errorf("%serror: %s: %s", prefix, op.Email, err)
	} else if result != ops.Unsubscribed {
		log.Printf("%sfailed: %s: %s", prefix, op.Email, result)
	} else {
		log.Printf("%ssuccess: %s", prefix, op.Email)
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

func (h *mailtoHandler) bounceIfDmarcFails(ev *mailtoEvent) (bool, error) {
	return false, nil
}

func isSpam(ev *mailtoEvent) bool {
	return false
}
