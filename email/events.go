package email

type SendEvent struct {
	Message
}

type SendResponse struct {
	Success bool
	NumSent int
	Details string
}
