package testdoubles

import (
	"context"
	"testing"
)

type Mailer struct {
	RecipientMessages map[string][]byte
	MessageIds        map[string]string
	RecipientErrors   map[string]error
	BulkCapError      error
}

func NewMailer() *Mailer {
	return &Mailer{
		RecipientMessages: make(map[string][]byte, 10),
		MessageIds:        make(map[string]string, 10),
		RecipientErrors:   make(map[string]error, 10),
	}
}

func (mailer *Mailer) BulkCapacityAvailable(_ context.Context) error {
	return mailer.BulkCapError
}

func (m *Mailer) Send(
	ctx context.Context, recipient string, msg []byte,
) (messageId string, err error) {
	if err = m.RecipientErrors[recipient]; err == nil {
		messageId = m.MessageIds[recipient]
		m.RecipientMessages[recipient] = msg
	}
	return
}

func (m *Mailer) GetMessageTo(
	t *testing.T, recipient string,
) (msgId, msg string) {
	t.Helper()
	var rawMsg []byte
	var ok bool

	if rawMsg, ok = m.RecipientMessages[recipient]; !ok {
		t.Fatalf("did not receive a message to %s", recipient)
	}
	return m.MessageIds[recipient], string(rawMsg)
}

func (m *Mailer) AssertNoMessageSent(t *testing.T, recipient string) {
	t.Helper()

	if msg, ok := m.RecipientMessages[recipient]; ok {
		t.Fatalf("expected %s to receive no messages, got: %s", recipient, msg)
	}
}
