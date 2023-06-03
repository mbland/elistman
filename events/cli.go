package events

import "github.com/mbland/elistman/email"

type CommandLineEventType string

const CommandLineSendEvent = CommandLineEventType("Send")

type CommandLineEvent struct {
	EListManCommand string     `json:"elistmanCommand"`
	Send            *SendEvent `json:"send"`
}

type SendEvent struct {
	email.Message
}

type SendResponse struct {
	Success bool
	NumSent int
	Details string
}
