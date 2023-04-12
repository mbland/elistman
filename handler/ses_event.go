// These types aren't defined in the AWS SDK. Note that not all event types are
// defined here; only the ones needed by this application, namely:
// - bounce
// - complaint
// - delivery
// - send
// - reject
//
// See:
// - https://docs.aws.amazon.com/ses/latest/dg/event-publishing-retrieving-sns-contents.html
// - https://docs.aws.amazon.com/ses/latest/dg/event-publishing-retrieving-sns-examples.html

package handler

import (
	"time"

	"github.com/aws/aws-lambda-go/events"
)

type SesEventRecord struct {
	EventType string             `json:"eventType"`
	Mail      SesEventMessage    `json:"mail"`
	Bounce    *SesBounceEvent    `json:"bounce"`
	Complaint *SesComplaintEvent `json:"complaint"`
	Delivery  *SesDeliveryEvent  `json:"delivery"`
	Send      *SesSendEvent      `json:"send"`
	Reject    *SesRejectEvent    `json:"reject"`
}

type SesEventMessage struct {
	events.SimpleEmailMessage
	SourceArn        string              `json:"sourceArn"`
	SendingAccountId string              `json:"sendingAccountId"`
	Tags             map[string][]string `json:"tags"`
}

type SesBounceEvent struct {
	BounceType        string                `json:"bounceType"`
	BounceSubType     string                `json:"bounceSubType"`
	BouncedRecipients []SesBouncedRecipient `json:"bouncedRecipients"`
	Timestamp         time.Time             `json:"timestamp"`
	FeedbackId        string                `json:"feedbackId"`
	ReportingMTA      string                `json:"reportingMTA"`
}

type SesBouncedRecipient struct {
	EmailAddress   string `json:"emailAddress"`
	Action         string `json:"action"`
	Status         string `json:"status"`
	DiagnosticCode string `json:"diagnosticCode"`
}

type SesComplaintEvent struct {
	ComplaintSubType      string                   `json:"complaintSubType"`
	ComplainedRecipients  []SesComplainedRecipient `json:"complainedRecipients"`
	Timestamp             time.Time                `json:"timestamp"`
	FeedbackId            string                   `json:"feedbackId"`
	UserAgent             string                   `json:"userAgent"`
	ComplaintFeedbackType string                   `json:"complaintFeedbackType"`
	ArrivalDate           time.Time                `json:"arrivalDate"`
}

type SesComplainedRecipient struct {
	EmailAddress string `json:"emailAddress"`
}

type SesDeliveryEvent struct {
	Timestamp            time.Time `json:"timestamp"`
	ProcessingTimeMillis int64     `json:"processingTimeMillis"`
	Recipients           []string  `json:"recipients"`
	SmtpResponse         string    `json:"smtpResponse"`
	ReportingMTA         string    `json:"reportingMTA"`
}

// According to the documentation, "The JSON object that contains information
// about a `send` event is always empty."
type SesSendEvent struct {
}

type SesRejectEvent struct {
	Reason string `json:"reason"`
}
