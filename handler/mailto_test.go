package handler

import (
	"strings"
	"testing"

	"github.com/mbland/elistman/ops"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestNewMailtoEvent(t *testing.T) {
	sesEvent := simpleEmailServiceEvent()

	expected := &mailtoEvent{
		From:         []string{"mbland@acm.org"},
		To:           []string{testUnsubscribeAddress},
		Subject:      "mbland@acm.org " + testValidUidStr,
		MessageId:    "deadbeef",
		SpfVerdict:   "PASS",
		DkimVerdict:  "PASS",
		SpamVerdict:  "PASS",
		VirusVerdict: "PASS",
		DmarcVerdict: "PASS",
		DmarcPolicy:  "REJECT",
	}

	assert.DeepEqual(t, expected, newMailtoEvent(sesEvent))
}

type mailtoHandlerFixture struct {
	agent   *testAgent
	logs    *strings.Builder
	handler *mailtoHandler
	event   *mailtoEvent
}

func newMailtoHandlerFixture() *mailtoHandlerFixture {
	logs, logger := testLogger()
	agent := &testAgent{}
	return &mailtoHandlerFixture{
		agent,
		logs,
		&mailtoHandler{"unsubscribe@mike-bland.com", agent, logger},
		newMailtoEvent(simpleEmailServiceEvent()),
	}
}

func TestHandleMailtoEvent(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.agent.ReturnValue = ops.Unsubscribed

		err := f.handler.handleMailtoEvent(f.event)

		assert.NilError(t, err)
		expectedLog := "unsubscribe message deadbeef: success: mbland@acm.org"
		assert.Assert(t, is.Contains(f.logs.String(), expectedLog))
	})

	t.Run("LogsParseErrors", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.event.Subject = "foo@bar.com UID"

		err := f.handler.handleMailtoEvent(f.event)

		assert.NilError(t, err)
		expectedLog := "unsubscribe message deadbeef: " +
			"failed to parse, ignoring: " +
			`[From:"mbland@acm.org" To:"unsubscribe@mike-bland.com" ` +
			`Subject:"foo@bar.com UID"]: `
		assert.Assert(t, is.Contains(f.logs.String(), expectedLog))
	})
}
