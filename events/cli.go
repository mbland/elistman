package events

import (
	"github.com/mbland/elistman/email"
)

type CommandLineEventType string

const (
	CommandLineSendEvent   = CommandLineEventType("Send")
	CommandLineImportEvent = CommandLineEventType("Import")
)

type CommandLineEvent struct {
	EListManCommand CommandLineEventType `json:"elistmanCommand"`
	Send            *SendEvent           `json:"send"`
	Import          *ImportEvent         `json:"import"`
}

type SendEvent struct {
	Addresses []string
	email.Message
}

type SendResponse struct {
	Success bool
	NumSent int
	Details string
}

type ImportEvent struct {
	Addresses []string
}

type ImportResponse struct {
	NumImported int
	Failures    []string
}
