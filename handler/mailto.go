package handler

import (
	"context"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/ops"
)

type mailtoHandler struct {
	EmailDomain     string
	UnsubscribeAddr string
	Agent           ops.SubscriptionAgent
	Bouncer         email.Bouncer
	Log             *log.Logger
}

func (h *mailtoHandler) HandleEvent(
	ctx context.Context, e *events.SimpleEmailEvent,
) *events.SimpleEmailDisposition {
	// If I understand the contract correctly, there should only ever be one
	// valid Record per event. However, we have the technology to deal
	// gracefully with the unexpected.
	for _, record := range e.Records {
		h.handleMailtoEvent(ctx, newMailtoEvent(&record.SES))
	}
	return &events.SimpleEmailDisposition{
		Disposition: events.SimpleEmailStopRuleSet,
	}
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
func (h *mailtoHandler) handleMailtoEvent(
	ctx context.Context, ev *mailtoEvent,
) {
	outcome := "success"
	unsubscribe := h.Agent.Unsubscribe

	if bounceMessageId, err := h.bounceIfDmarcFails(ctx, ev); err != nil {
		outcome = "DMARC bounce failed: " + err.Error()
	} else if bounceMessageId != "" {
		outcome = "DMARC bounced with message ID: " + bounceMessageId
	} else if isSpam(ev) {
		outcome = "marked as spam, ignored"
	} else if op, err := parseMailtoEvent(ev, h.UnsubscribeAddr); err != nil {
		outcome = "failed to parse, ignoring: " + err.Error()
	} else if result, err := unsubscribe(ctx, op.Email, op.Uid); err != nil {
		outcome = "error: " + err.Error()
	} else if result != ops.Unsubscribed {
		outcome = "failed: " + result.String()
	}
	h.logOutcome(ev, outcome)
}

func (h *mailtoHandler) logOutcome(ev *mailtoEvent, outcome string) {
	h.Log.Printf(
		`unsubscribe [Id:"%s" From:"%s" To:"%s" Subject:"%s"]: %s`,
		ev.MessageId,
		strings.Join(ev.From, ","),
		strings.Join(ev.To, ","),
		ev.Subject,
		outcome,
	)
}

func (h *mailtoHandler) bounceIfDmarcFails(
	ctx context.Context, ev *mailtoEvent,
) (bounceMessageId string, err error) {
	if ev.DmarcVerdict == "FAIL" && ev.DmarcPolicy == "REJECT" {
		bounceMessageId, err = h.Bouncer.Bounce(
			ctx, h.EmailDomain, ev.Recipients, ev.Timestamp,
		)
	}
	return
}

func isSpam(ev *mailtoEvent) bool {
	return ev.SpfVerdict == "FAIL" ||
		ev.DkimVerdict == "FAIL" ||
		ev.SpamVerdict == "FAIL" ||
		ev.VirusVerdict == "FAIL"
}
