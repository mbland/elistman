package testdoubles

import (
	"context"
	"testing"
)

type Mailer struct {
	RecipientMessages map[string][]byte
	MessageIds        map[string]string
	RecipientErrors   map[string]error
}

func NewMailer() *Mailer {
	return &Mailer{
		RecipientMessages: make(map[string][]byte, 10),
		MessageIds:        make(map[string]string, 10),
		RecipientErrors:   make(map[string]error, 10),
	}
}

func (m *Mailer) Send(
	ctx context.Context, recipient string, msg []byte,
) (messageId string, err error) {
	m.RecipientMessages[recipient] = msg
	return m.MessageIds[recipient], m.RecipientErrors[recipient]
}

func (m *Mailer) GetMessageTo(t *testing.T, recipient string) string {
	t.Helper()
	var msg []byte
	var ok bool

	if msg, ok = m.RecipientMessages[recipient]; !ok {
		t.Fatalf("did not receive a message to %s", recipient)
	}
	return string(msg)
}
