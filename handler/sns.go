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
	handler = &sesEventHandler{Event: event, Details: message}
	return
}

type sesEventHandler struct {
	Event   *events.SesEventRecord
	Details string
	Agent   agent.SubscriptionAgent
	Log     *log.Logger
}

func (evh *sesEventHandler) HandleEvent(ctx context.Context) {
	event := evh.Event
	switch evh.Event.EventType {
	case "Bounce":
		evh.handleBounceEvent(ctx)
	case "Complaint":
		evh.handleComplaintEvent(ctx)
	case "Reject":
		evh.logOutcome(event.Reject.Reason)
	case "Send", "Delivery":
		evh.logOutcome("success")
	default:
		evh.Log.Printf("unimplemented event type: %s", event.EventType)
	}
}

func (evh *sesEventHandler) logOutcome(outcome string) {
	event := evh.Event
	headers := &event.Mail.CommonHeaders

	evh.Log.Printf(
		`%s [Id:"%s" From:"%s" To:"%s" Subject:"%s"]: %s: %s`,
		evh.Event.EventType,
		event.Mail.MessageID,
		strings.Join(headers.From, ","),
		strings.Join(headers.To, ","),
		headers.Subject,
		outcome,
		evh.Details,
	)
}

func (evh *sesEventHandler) removeRecipients(
	ctx context.Context, reason string,
) {
	remove := evh.Agent.Remove
	evh.updateRecipients(ctx, reason, remove, "removed", "error removing")
}

func (evh *sesEventHandler) restoreRecipients(
	ctx context.Context, reason string,
) {
	restore := evh.Agent.Restore
	evh.updateRecipients(ctx, reason, restore, "restored", "error restoring")
}

func (evh *sesEventHandler) updateRecipients(
	ctx context.Context,
	reason string,
	action func(context.Context, string) error,
	successPrefix, errPrefix string,
) {
	for _, email := range evh.Event.Mail.CommonHeaders.To {
		emailAndReason := " " + email + " due to: " + reason
		outcome := successPrefix + emailAndReason

		if err := action(ctx, email); err != nil {
			outcome = errPrefix + emailAndReason + ": " + err.Error()
		}
		evh.logOutcome(outcome)
	}
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
