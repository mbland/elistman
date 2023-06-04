package handler

import (
	"context"
	"encoding/json"
	"fmt"
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
		if sesHandler, err := newSesEventHandler(h, msg); err != nil {
			h.Log.Printf("parsing SES event from SNS failed: %s: %s", err, msg)
		} else {
			sesHandler.HandleEvent(ctx)
		}
	}
}

type sesEventHandler interface {
	HandleEvent(ctx context.Context)
}

func parseSesEvent(
	sns *snsHandler, message string,
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
		Agent:     sns.Agent,
		Log:       sns.Log,
	}
	return
}

func newSesEventHandler(
	sns *snsHandler, message string,
) (handler sesEventHandler, err error) {
	event, base, err := parseSesEvent(sns, message)
	if err != nil {
		return
	}

	switch base.Type {
	case "Bounce":
		handler = &bounceHandler{
			baseSesEventHandler: *base,
			BounceType:          event.Bounce.BounceType,
			BounceSubType:       event.Bounce.BounceSubType,
		}
	case "Complaint":
		handler = &complaintHandler{
			baseSesEventHandler:   *base,
			ComplaintSubType:      event.Complaint.ComplaintSubType,
			ComplaintFeedbackType: event.Complaint.ComplaintFeedbackType,
		}
	case "Reject":
		handler = &rejectHandler{
			baseSesEventHandler: *base,
			Reason:              event.Reject.Reason,
		}
	case "Send", "Delivery":
		handler = base
	default:
		err = fmt.Errorf("unimplemented event type: %s", base.Type)
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

func (evh *baseSesEventHandler) HandleEvent(ctx context.Context) {
	evh.logOutcome("success")
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
