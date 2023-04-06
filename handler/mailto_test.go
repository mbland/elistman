package handler

import (
	"strings"
	"testing"
	"time"

	"github.com/mbland/elistman/ops"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type mailtoHandlerFixture struct {
	agent   *testAgent
	logs    *strings.Builder
	handler *mailtoHandler
	event   *mailtoEvent
}

func newMailtoHandlerFixture() *mailtoHandlerFixture {
	logs, logger := testLogger()
	agent := &testAgent{}
	timestamp, err := time.Parse(time.DateOnly, "1970-09-18")

	if err != nil {
		panic("failed to parse mailtoHandlerFixture timestamp: " + err.Error())
	}

	return &mailtoHandlerFixture{
		agent,
		logs,
		newMailtoHandler(testEmailDomain, agent, logger),
		&mailtoEvent{
			From:         []string{"mbland@acm.org"},
			To:           []string{testUnsubscribeAddress},
			Subject:      "mbland@acm.org " + testValidUidStr,
			Recipients:   []string{testUnsubscribeAddress},
			Timestamp:    timestamp,
			MessageId:    "deadbeef",
			SpfVerdict:   "PASS",
			DkimVerdict:  "PASS",
			SpamVerdict:  "PASS",
			VirusVerdict: "PASS",
			DmarcVerdict: "PASS",
			DmarcPolicy:  "REJECT",
		},
	}
}

func TestNewMailtoEvent(t *testing.T) {
	f := newMailtoHandlerFixture()

	assert.DeepEqual(t, f.event, newMailtoEvent(simpleEmailService()))
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
