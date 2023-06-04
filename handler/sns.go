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
		event, handler, err := parseSesEvent(msg)

		if err != nil {
			h.Log.Printf("parsing SES event from SNS failed: %s: %s", err, msg)
			continue
		}
		handler.Agent = h.Agent
		handler.Log = h.Log
		handler.HandleEvent(ctx, event)
	}
}

func parseSesEvent(
	message string,
) (event *events.SesEventRecord, handler *baseSesEventHandler, err error) {
	event = &events.SesEventRecord{}
	if err = json.Unmarshal([]byte(message), event); err != nil {
		event = nil
		return
	}

	mail := event.Mail
	handler = &baseSesEventHandler{
		Type:      event.EventType,
		MessageId: mail.MessageID,
		To:        mail.CommonHeaders.To,
		From:      mail.CommonHeaders.From,
		Subject:   mail.CommonHeaders.Subject,
		Details:   message,
	}
	return
}

type baseSesEventHandler struct {
	Type      string
	MessageId string
	From      []string
	To        []string
	Subject   string
	Details   string
	Agent     agent.SubscriptionAgent
	Log       *log.Logger
}

func (evh *baseSesEventHandler) HandleEvent(
	ctx context.Context, event *events.SesEventRecord,
) {
	switch event.EventType {
	case "Bounce":
		handler := &bounceHandler{
			baseSesEventHandler: *evh,
			BounceType:          event.Bounce.BounceType,
			BounceSubType:       event.Bounce.BounceSubType,
		}
		handler.HandleEvent(ctx)
	case "Complaint":
		handler := &complaintHandler{
			baseSesEventHandler:   *evh,
			ComplaintSubType:      event.Complaint.ComplaintSubType,
			ComplaintFeedbackType: event.Complaint.ComplaintFeedbackType,
		}
		handler.HandleEvent(ctx)
	case "Reject":
		handler := &rejectHandler{
			baseSesEventHandler: *evh,
			Reason:              event.Reject.Reason,
		}
		handler.HandleEvent(ctx)
	case "Send", "Delivery":
		evh.logOutcome("success")
	default:
		evh.Log.Printf("unimplemented event type: %s", event.EventType)
	}
}

func (evh *baseSesEventHandler) logOutcome(outcome string) {
	evh.Log.Printf(
		`%s [Id:"%s" From:"%s" To:"%s" Subject:"%s"]: %s: %s`,
		evh.Type,
		evh.MessageId,
		strings.Join(evh.From, ","),
		strings.Join(evh.To, ","),
		evh.Subject,
		outcome,
		evh.Details,
	)
}

func (evh *baseSesEventHandler) removeRecipients(
	ctx context.Context, reason string,
) {
	evh.updateRecipients(
		ctx,
		reason,
		&recipientUpdater{evh.Agent.Remove, "removed", "error removing"},
	)
}

func (evh *baseSesEventHandler) restoreRecipients(
	ctx context.Context, reason string,
) {
	evh.updateRecipients(
		ctx,
		reason,
		&recipientUpdater{evh.Agent.Restore, "restored", "error restoring"},
	)
}

func (evh *baseSesEventHandler) updateRecipients(
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

type bounceHandler struct {
	baseSesEventHandler
	BounceType    string
	BounceSubType string
}

func (evh *bounceHandler) HandleEvent(ctx context.Context) {
	reason := evh.BounceType + "/" + evh.BounceSubType
	if evh.BounceType == "Transient" {
		evh.logOutcome("not removing recipients: " + reason)
	} else {
		evh.removeRecipients(ctx, reason)
	}
}

type complaintHandler struct {
	baseSesEventHandler
	ComplaintSubType      string
	ComplaintFeedbackType string
}

func (evh *complaintHandler) HandleEvent(ctx context.Context) {
	reason := evh.ComplaintSubType
	if reason == "" {
		reason = evh.ComplaintFeedbackType
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

type rejectHandler struct {
	baseSesEventHandler
	Reason string
}

func (evh *rejectHandler) HandleEvent(ctx context.Context) {
	evh.logOutcome(evh.Reason)
}
