package events

import "github.com/mbland/elistman/email"

type SendEvent struct {
	email.Message
}

type SendResponse struct {
	Success bool
	NumSent int
	Details string
}
