package handler

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mbland/elistman/ops"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type mailtoHandlerFixture struct {
	agent   *testAgent
	bouncer *testBouncer
	logs    *strings.Builder
	handler *mailtoHandler
	event   *mailtoEvent
}

func newMailtoHandlerFixture() *mailtoHandlerFixture {
	logs, logger := testLogger()
	agent := &testAgent{}
	bouncer := &testBouncer{}
	bouncer.ReturnMessageId = "0x123456789"
	timestamp, err := time.Parse(time.DateOnly, "1970-09-18")

	if err != nil {
		panic("failed to parse mailtoHandlerFixture timestamp: " + err.Error())
	}

	return &mailtoHandlerFixture{
		agent,
		bouncer,
		logs,
		newMailtoHandler(testEmailDomain, agent, bouncer, logger),
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

func TestBounceIfDmarcFails(t *testing.T) {
	t.Run("DoesNothingIfDoesNotFail", func(t *testing.T) {
		f := newMailtoHandlerFixture()

		bounceMessageId, err := f.handler.bounceIfDmarcFails(f.event)

		assert.NilError(t, err)
		assert.Equal(t, "", bounceMessageId)
	})

	t.Run("DoesNothingIfPolicyIsNotREJECT", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.event.DmarcVerdict = "FAIL"
		f.event.DmarcPolicy = "NONE"

		bounceMessageId, err := f.handler.bounceIfDmarcFails(f.event)

		assert.NilError(t, err)
		assert.Equal(t, "", bounceMessageId)
	})

	t.Run("BouncesIfStatusIsFAILAndPolicyIsREJECT", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.event.DmarcVerdict = "FAIL"
		f.event.DmarcPolicy = "REJECT"

		bounceMessageId, err := f.handler.bounceIfDmarcFails(f.event)

		assert.NilError(t, err)
		assert.Equal(t, "0x123456789", bounceMessageId)
	})

	t.Run("ReturnsErrorIfBounceFails", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.event.DmarcVerdict = "FAIL"
		f.event.DmarcPolicy = "REJECT"
		f.bouncer.Error = errors.New("couldn't bounce")

		bounceMessageId, err := f.handler.bounceIfDmarcFails(f.event)

		assert.Equal(t, "", bounceMessageId)
		assert.ErrorContains(t, err, "couldn't bounce")
	})
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
