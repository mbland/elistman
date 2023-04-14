//go:build small_tests || all_tests

package handler

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/mbland/elistman/ops"
	"gotest.tools/assert"
)

type mailtoHandlerFixture struct {
	agent   *testAgent
	bouncer *testBouncer
	logs    *strings.Builder
	handler *mailtoHandler
	event   *mailtoEvent
}

func (f *mailtoHandlerFixture) Logs() string {
	return f.logs.String()
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
		&mailtoHandler{
			testEmailDomain, testUnsubscribeAddress, agent, bouncer, logger,
		},
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

func TestLogOutcome(t *testing.T) {
	// Though normally we only expect one From: and one To: address, we include
	// multiple of each to ensure joining is happening.
	f := newMailtoHandlerFixture()
	f.event.From = append(f.event.From, "foo@bar.com")
	f.event.To = append(f.event.To, "baz@quux.com")

	f.handler.logOutcome(f.event, "success")

	assertLogsContain(t, f, `unsubscribe [Id:"deadbeef" `+
		`From:"mbland@acm.org,foo@bar.com" `+
		`To:"`+testUnsubscribeAddress+`,baz@quux.com" `+
		`Subject:"mbland@acm.org `+testValidUidStr+`"]: success`)
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
		assert.Equal(t, testEmailDomain, f.bouncer.EmailDomain)
		assert.DeepEqual(t, f.event.Recipients, f.bouncer.Recipients)
		assert.Equal(t, f.event.Timestamp, f.bouncer.Timestamp)
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

func TestIsSpam(t *testing.T) {
	t.Run("ReturnsFalseIfNoVerdictsFail", func(t *testing.T) {
		assert.Assert(t, !isSpam(&mailtoEvent{}))
	})

	t.Run("ReturnsTrueIfAnyVerdictFails", func(t *testing.T) {
		assert.Check(t, isSpam(&mailtoEvent{SpfVerdict: "FAIL"}))
		assert.Check(t, isSpam(&mailtoEvent{DkimVerdict: "FAIL"}))
		assert.Check(t, isSpam(&mailtoEvent{SpamVerdict: "FAIL"}))
		assert.Assert(t, isSpam(&mailtoEvent{VirusVerdict: "FAIL"}))
	})
}

func TestHandleMailtoEvent(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.agent.ReturnValue = ops.Unsubscribed

		f.handler.handleMailtoEvent(f.event)

		assertLogsContain(t, f, `unsubscribe [Id:"deadbeef" `+
			`From:"mbland@acm.org" `+
			`To:"`+testUnsubscribeAddress+`" `+
			`Subject:"mbland@acm.org `+testValidUidStr+`"]: success`)
	})

	t.Run("LogsIfFailsToBounceOnDmarcFail", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.event.DmarcVerdict = "FAIL"
		f.event.DmarcPolicy = "REJECT"
		f.bouncer.Error = errors.New("couldn't bounce")

		f.handler.handleMailtoEvent(f.event)

		assertLogsContain(t, f, "DMARC bounce failed: couldn't bounce")
	})

	t.Run("BouncesOnDmarcFail", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.event.DmarcVerdict = "FAIL"
		f.event.DmarcPolicy = "REJECT"
		f.bouncer.ReturnMessageId = "0x123456789"

		f.handler.handleMailtoEvent(f.event)

		assertLogsContain(t, f, "DMARC bounced with message ID: 0x123456789")
	})

	t.Run("IgnoresIfSpam", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.event.VirusVerdict = "FAIL"

		f.handler.handleMailtoEvent(f.event)

		assertLogsContain(t, f, "marked as spam, ignored")
	})

	t.Run("LogsParseErrors", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.event.Subject = "foo@bar.com UID"

		f.handler.handleMailtoEvent(f.event)

		assertLogsContain(t, f, `failed to parse, ignoring: invalid uid: `)
	})

	t.Run("LogsIfUnsubscribeErrors", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.agent.Error = errors.New("agent failed")

		f.handler.handleMailtoEvent(f.event)

		assertLogsContain(t, f, `error: agent failed`)
	})

	t.Run("LogsIfUnsubscribeFails", func(t *testing.T) {
		f := newMailtoHandlerFixture()
		f.agent.ReturnValue = ops.Invalid

		f.handler.handleMailtoEvent(f.event)

		assertLogsContain(t, f, `failed: Invalid`)
	})
}

func TestMailtoHandlerHandleEvent(t *testing.T) {
	f := newMailtoHandlerFixture()
	f.agent.ReturnValue = ops.Unsubscribed

	response := f.handler.HandleEvent(simpleEmailEvent())

	expected := &events.SimpleEmailDisposition{
		Disposition: events.SimpleEmailStopRuleSet,
	}
	assert.DeepEqual(t, expected, response)
	assertLogsContain(t, f, "success")
	assert.Equal(t, "mbland@acm.org", f.agent.Email)
	assert.Equal(t, testValidUid, f.agent.Uid)
}
