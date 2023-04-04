package handler

import (
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"gotest.tools/assert"
)

func TestNewMailtoEvent(t *testing.T) {
	from := []string{"mbland@acm.org"}
	to := []string{testUnsubscribeAddress}
	subject := from[0] + " 0123-456-789"
	const messageId = "deadbeef"
	const spfVerdict = "PASS"
	const dkimVerdict = "PASS"
	const spamVerdict = "PASS"
	const virusVerdict = "PASS"
	const dmarcVerdict = "PASS"
	const dmarcPolicy = "REJECT"

	sesEvent := &events.SimpleEmailService{
		Mail: events.SimpleEmailMessage{
			MessageID: messageId,
			CommonHeaders: events.SimpleEmailCommonHeaders{
				From:    from,
				To:      to,
				Subject: subject,
			},
		},
		Receipt: events.SimpleEmailReceipt{
			SPFVerdict:   events.SimpleEmailVerdict{Status: spfVerdict},
			DKIMVerdict:  events.SimpleEmailVerdict{Status: dkimVerdict},
			SpamVerdict:  events.SimpleEmailVerdict{Status: spamVerdict},
			VirusVerdict: events.SimpleEmailVerdict{Status: virusVerdict},
			DMARCVerdict: events.SimpleEmailVerdict{Status: dmarcVerdict},
			DMARCPolicy:  dmarcPolicy,
		},
	}

	expected := &mailtoEvent{
		from, to, subject, messageId,
		spfVerdict, dkimVerdict, spamVerdict, virusVerdict,
		dmarcVerdict, dmarcPolicy,
	}

	assert.DeepEqual(t, expected, newMailtoEvent(sesEvent))
}

func newTestMailtoHandler() *mailtoHandler {
	return &mailtoHandler{"unsubscribe@mike-bland.com", &testAgent{}}
}

func TestMailtoEventDoesNothingUntilImplemented(t *testing.T) {
	handler := newTestMailtoHandler()
	_, teardown := captureLogs()
	defer teardown()

	err := handler.handleMailtoEvent(&mailtoEvent{
		To:      []string{"unsubscribe@mike-bland.com"},
		Subject: "foo@bar.com UID",
	})

	assert.NilError(t, err)
}
