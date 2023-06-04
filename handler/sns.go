package handler

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/agent"
	"github.com/mbland/elistman/events"
)

type snsHandler struct {
	Agent agent.SubscriptionAgent
	Log   *log.Logger
}

// https://docs.aws.amazon.com/ses/latest/dg/event-publishing-retrieving-sns-contents.html
// https://docs.aws.amazon.com/ses/latest/dg/event-publishing-retrieving-sns-examples.html
func (h *snsHandler) HandleEvent(ctx context.Context, e *awsevents.SNSEvent) {
	for _, snsRecord := range e.Records {
		msg := snsRecord.SNS.Message
		handler, err := parseSesEvent(msg)

		if err != nil {
			h.Log.Printf("parsing SES event from SNS failed: %s: %s", err, msg)
			continue
		}
		handler.Agent = h.Agent
		handler.Log = h.Log
		handler.HandleEvent(ctx)
	}
}

func parseSesEvent(message string) (handler *sesEventHandler, err error) {
	event := &events.SesEventRecord{}
	if err = json.Unmarshal([]byte(message), event); err != nil {
		event = nil
		return
	}

	mail := event.Mail
	handler = &sesEventHandler{
		Event:     event,
		MessageId: mail.MessageID,
		To:        mail.CommonHeaders.To,
		From:      mail.CommonHeaders.From,
		Subject:   mail.CommonHeaders.Subject,
		Details:   message,
	}
	return
}

type sesEventHandler struct {
	Event     *events.SesEventRecord
	MessageId string
	From      []string
	To        []string
	Subject   string
	Details   string
	Agent     agent.SubscriptionAgent
	Log       *log.Logger
}

func (evh *sesEventHandler) HandleEvent(ctx context.Context) {
	event := evh.Event
	switch event.EventType {
	case "Bounce":
		evh.handleBounceEvent(ctx)
	case "Complaint":
		evh.handleComplaintEvent(ctx)
	case "Reject":
		evh.logOutcome(evh.Event.Reject.Reason)
	case "Send", "Delivery":
		evh.logOutcome("success")
	default:
		evh.Log.Printf("unimplemented event type: %s", event.EventType)
	}
}

func (evh *sesEventHandler) logOutcome(outcome string) {
	evh.Log.Printf(
		`%s [Id:"%s" From:"%s" To:"%s" Subject:"%s"]: %s: %s`,
		evh.Event.EventType,
		evh.MessageId,
		strings.Join(evh.From, ","),
		strings.Join(evh.To, ","),
		evh.Subject,
		outcome,
		evh.Details,
	)
}

func (evh *sesEventHandler) removeRecipients(
	ctx context.Context, reason string,
) {
	op := &recipientUpdater{evh.Agent.Remove, "removed", "error removing"}
	evh.updateRecipients(ctx, reason, op)
}

func (evh *sesEventHandler) restoreRecipients(ctx context.Context, reason string,
) {
	op := &recipientUpdater{evh.Agent.Restore, "restored", "error restoring"}
	evh.updateRecipients(ctx, reason, op)
}

func (evh *sesEventHandler) updateRecipients(
	ctx context.Context, reason string, up *recipientUpdater,
) {
	for _, email := range evh.To {
		evh.logOutcome(up.updateRecipient(ctx, email, reason))
	}
}

type recipientUpdater struct {
	action        func(context.Context, string) error
	successPrefix string
	errPrefix     string
}

func (up *recipientUpdater) updateRecipient(
	ctx context.Context, email, reason string,
) string {
	emailAndReason := " " + email + " due to: " + reason

	if err := up.action(ctx, email); err != nil {
		return up.errPrefix + emailAndReason + ": " + err.Error()
	}
	return up.successPrefix + emailAndReason
}

func (evh *sesEventHandler) handleBounceEvent(ctx context.Context) {
	event := evh.Event.Bounce
	reason := event.BounceType + "/" + event.BounceSubType
	if event.BounceType == "Transient" {
		evh.logOutcome("not removing recipients: " + reason)
	} else {
		evh.removeRecipients(ctx, reason)
	}
}

func (evh *sesEventHandler) handleComplaintEvent(ctx context.Context) {
	event := evh.Event.Complaint
	reason := event.ComplaintSubType
	if reason == "" {
		reason = event.ComplaintFeedbackType
	}
	if reason == "" {
		reason = "unknown"
	}

	if reason == "not-spam" {
		evh.restoreRecipients(ctx, reason)
	} else {
		evh.removeRecipients(ctx, reason)
	}
}
