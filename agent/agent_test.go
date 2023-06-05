//go:build small_tests || all_tests

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/ops"
	td "github.com/mbland/elistman/testdata"
	"github.com/mbland/elistman/testdoubles"
	tu "github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

const testEmail = td.TestEmail
const testSender = "Blog Updates <updates@foo.com>"
const testSiteTitle = "Foo Blog"
const testDomainName = "foo.com"
const testUnsubEmail = "unsubscribe@foo.com"
const testUnsubBaseUrl = "https://foo.com/email/"

var pendingSubscriber *db.Subscriber = &db.Subscriber{
	Email:     testEmail,
	Uid:       td.TestUid,
	Status:    db.SubscriberPending,
	Timestamp: td.TestTimestamp.Add(timeToLiveDuration),
}

var verifiedSubscriber *db.Subscriber = &db.Subscriber{
	Email:     testEmail,
	Uid:       uuid.MustParse("55555555-6666-7777-8888-999999999999"),
	Status:    db.SubscriberVerified,
	Timestamp: td.TestTimestamp,
}

type prodAgentTestFixture struct {
	agent      *ProdAgent
	db         *testdoubles.Database
	validator  *testdoubles.AddressValidator
	mailer     *testdoubles.Mailer
	suppressor *testdoubles.Suppressor
	logs       *tu.Logs
}

func newProdAgentTestFixture() *prodAgentTestFixture {
	newUid := func() (uuid.UUID, error) {
		return td.TestUid, nil
	}
	currentTime := func() time.Time {
		return td.TestTimestamp
	}
	db := testdoubles.NewDatabase()
	av := testdoubles.NewAddressValidator()
	m := testdoubles.NewMailer()
	sup := testdoubles.NewSuppressor()
	logs, logger := tu.NewLogs()
	pa := &ProdAgent{
		testSender,
		testSiteTitle,
		testDomainName,
		testUnsubEmail,
		testUnsubBaseUrl,
		newUid,
		currentTime,
		db,
		av,
		m,
		sup,
		logger,
	}
	return &prodAgentTestFixture{pa, db, av, m, sup, logs}
}

func makeServerError(msg string) error {
	return ops.AwsError("test error", tu.AwsServerError(msg))
}

func assertServerErrorContains(t *testing.T, err error, expectedMsg string) {
	t.Helper()
	assert.ErrorContains(t, err, expectedMsg)
	assert.Assert(t, tu.ErrorIs(err, ops.ErrExternal))
}

func TestPutSubscriber(t *testing.T) {
	setup := func() (
		*ProdAgent, *testdoubles.Database, *db.Subscriber, context.Context,
	) {
		f := newProdAgentTestFixture()
		sub := &db.Subscriber{Email: testEmail, Status: db.SubscriberPending}
		return f.agent, f.db, sub, context.Background()
	}

	t.Run("Succeeds", func(t *testing.T) {
		agent, dbase, sub, ctx := setup()

		err := agent.putSubscriber(ctx, sub)

		assert.NilError(t, err)
		assert.DeepEqual(t, pendingSubscriber, sub)
		assert.DeepEqual(t, pendingSubscriber, dbase.Index[sub.Email])
	})

	t.Run("ReturnsErrorIfNewUidFails", func(t *testing.T) {
		agent, dbase, sub, ctx := setup()
		agent.NewUid = func() (uuid.UUID, error) {
			return uuid.Nil, errors.New("NewUid failed")
		}

		err := agent.putSubscriber(ctx, sub)

		assert.Error(t, err, "NewUid failed")
		assert.Assert(t, is.Nil(dbase.Index[sub.Email]))
	})

	t.Run("PassesThroughPutError", func(t *testing.T) {
		agent, dbase, sub, ctx := setup()
		dbase.SimulatePutErr = func(address string) error {
			return makeServerError("error while putting " + address)
		}

		err := agent.putSubscriber(ctx, sub)

		assertServerErrorContains(t, err, "error while putting "+sub.Email)
	})
}

func TestValidate(t *testing.T) {
	setup := func() (
		agent *ProdAgent,
		validator *testdoubles.AddressValidator,
		logs *tu.Logs,
	) {
		f := newProdAgentTestFixture()
		return f.agent, f.validator, f.logs
	}

	ctx := context.Background()

	t.Run("Succeeds", func(t *testing.T) {
		agent, validator, logs := setup()

		ok, err := agent.Validate(ctx, testEmail)

		assert.NilError(t, err)
		assert.Assert(t, ok == true)
		validator.AssertValidated(t, testEmail)
		assert.Equal(t, "", logs.Builder.String())
	})

	t.Run("FailsIfValidationFails", func(t *testing.T) {
		agent, validator, logs := setup()
		validator.Failure = &email.ValidationFailure{
			Address: testEmail, Reason: "test failure",
		}

		ok, err := agent.Validate(ctx, testEmail)

		assert.NilError(t, err)
		assert.Assert(t, ok == false)
		validator.AssertValidated(t, testEmail)
		logs.AssertContains(t, "validation failed: "+testEmail+": test failure")
	})

	t.Run("PassesThroughError", func(t *testing.T) {
		agent, validator, logs := setup()
		validator.Error = makeServerError("test error")

		ok, err := agent.Validate(ctx, testEmail)

		assert.Assert(t, ok == false)
		validator.AssertValidated(t, testEmail)
		assert.Equal(t, "", logs.Builder.String())
		assertServerErrorContains(t, err, "test error")
	})
}

func TestMakeVerificationEmail(t *testing.T) {
	setup := func() *ProdAgent {
		f := newProdAgentTestFixture()
		return f.agent
	}

	sub := pendingSubscriber

	t.Run("Succeeds", func(t *testing.T) {
		agent := setup()

		rawMsg := agent.makeVerificationEmail(sub)

		msg, _, pr := tu.ParseMultipartMessageAndBoundary(t, string(rawMsg))
		th := tu.TestHeader{Header: msg.Header}
		th.Assert(t, "From", agent.SenderAddress)
		th.Assert(t, "To", sub.Email)
		th.Assert(t, "Subject", verifySubjectPrefix+agent.EmailSiteTitle)

		verifyLink := ops.VerifyUrl(agent.ApiBaseUrl, sub.Email, sub.Uid)
		textPart := tu.GetNextPartContent(t, pr, "text/plain")
		assert.Assert(t, is.Contains(textPart, agent.EmailSiteTitle))
		assert.Assert(t, is.Contains(textPart, verifyLink))

		htmlPart := tu.GetNextPartContent(t, pr, "text/html")
		assert.Assert(t, is.Contains(htmlPart, agent.EmailSiteTitle))
		verifyAnchor := "<a href=\"" + verifyLink + "\">" + verifyLink + "</a>"
		assert.Assert(t, is.Contains(htmlPart, verifyAnchor))
	})
}

func TestSubscribe(t *testing.T) {
	setup := func() (*prodAgentTestFixture, context.Context) {
		return newProdAgentTestFixture(), context.Background()
	}

	t.Run("CreatesNewSubscriberAndSendsVerificationEmail", func(t *testing.T) {
		f, ctx := setup()
		msgId := "deadbeef"
		f.mailer.MessageIds[testEmail] = msgId

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.NilError(t, err)
		assert.Equal(t, ops.VerifyLinkSent, result)
		f.validator.AssertValidated(t, testEmail)

		sub := f.db.Index[testEmail]
		assert.DeepEqual(t, pendingSubscriber, sub)

		sentMsgId, verifyEmail := f.mailer.GetMessageTo(t, testEmail)
		assert.Equal(t, msgId, sentMsgId)
		assert.Assert(t, is.Contains(verifyEmail, verifySubjectPrefix))

		expectedLog := "sent verification email to " + testEmail +
			" with ID " + msgId
		f.logs.AssertContains(t, expectedLog)
	})

	t.Run("ReturnsVerifyLinkSentForPendingSubscribers", func(t *testing.T) {
		f, ctx := setup()
		assert.NilError(t, f.db.Put(ctx, pendingSubscriber))

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.NilError(t, err)
		assert.Equal(t, ops.VerifyLinkSent, result)
		f.mailer.AssertNoMessageSent(t, testEmail)
	})

	t.Run("ReturnsAlreadySubscribedForVerifiedSubscribers", func(t *testing.T) {
		f, ctx := setup()
		assert.NilError(t, f.db.Put(ctx, verifiedSubscriber))

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.NilError(t, err)
		assert.Equal(t, ops.AlreadySubscribed, result)
		f.mailer.AssertNoMessageSent(t, testEmail)
	})

	t.Run("ReturnsInvalidIfAddressFailsValidation", func(t *testing.T) {
		f, ctx := setup()
		f.validator.Failure = &email.ValidationFailure{
			Address: testEmail, Reason: "testing",
		}

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.NilError(t, err)
		assert.Equal(t, ops.Invalid, result)
		f.mailer.AssertNoMessageSent(t, testEmail)
		f.logs.AssertContains(t, "validation failed: "+testEmail+": testing")
	})

	t.Run("PassesThroughValidateAddressError", func(t *testing.T) {
		f, ctx := setup()
		f.validator.Error = makeServerError("SES error")

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.Equal(t, ops.Invalid, result)
		assertServerErrorContains(t, err, "SES error")
		f.mailer.AssertNoMessageSent(t, testEmail)
	})

	t.Run("PassesThroughGetError", func(t *testing.T) {
		f, ctx := setup()
		f.db.SimulateGetErr = func(email string) error {
			return makeServerError("error getting " + email)
		}

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.Equal(t, ops.Invalid, result)
		assertServerErrorContains(t, err, "error getting "+testEmail)
		f.mailer.AssertNoMessageSent(t, testEmail)
	})

	t.Run("PassesThroughPutError", func(t *testing.T) {
		f, ctx := setup()
		f.db.SimulatePutErr = func(email string) error {
			return makeServerError("error putting " + email)
		}

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.Equal(t, ops.Invalid, result)
		assertServerErrorContains(t, err, "error putting "+testEmail)
		f.mailer.AssertNoMessageSent(t, testEmail)
	})

	t.Run("PassesThroughSendError", func(t *testing.T) {
		f, ctx := setup()
		f.mailer.RecipientErrors[testEmail] = makeServerError("send failed")

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.Equal(t, ops.Invalid, result)
		assertServerErrorContains(t, err, "send failed")
	})
}

func TestGetSubscriber(t *testing.T) {
	setup := func() (*ProdAgent, *testdoubles.Database, context.Context) {
		f := newProdAgentTestFixture()
		return f.agent, f.db, context.Background()
	}

	t.Run("SucceedsWhenSubscriberExists", func(t *testing.T) {
		agent, dbase, ctx := setup()
		assert.NilError(t, dbase.Put(ctx, pendingSubscriber))

		sub, err := agent.getSubscriber(ctx, testEmail, td.TestUid)

		assert.NilError(t, err)
		assert.DeepEqual(t, pendingSubscriber, sub)
	})

	t.Run("ReturnsNilSubscriberAndNilErrorIfNotFound", func(t *testing.T) {
		agent, _, ctx := setup()

		sub, err := agent.getSubscriber(ctx, testEmail, td.TestUid)

		assert.NilError(t, err)
		assert.Assert(t, is.Nil(sub))
	})

	t.Run("ReturnsNilSubscriberAndNilErrorIfWrongUid", func(t *testing.T) {
		agent, dbase, ctx := setup()
		assert.NilError(t, dbase.Put(ctx, pendingSubscriber))
		wrongUid := uuid.MustParse("11111111-2222-3333-5555-888888888888")

		sub, err := agent.getSubscriber(ctx, testEmail, wrongUid)

		assert.NilError(t, err)
		assert.Assert(t, is.Nil(sub))
	})

	t.Run("PassesThroughServerError", func(t *testing.T) {
		agent, dbase, ctx := setup()
		dbase.SimulateGetErr = func(address string) error {
			return makeServerError("error getting " + address)
		}

		sub, err := agent.getSubscriber(ctx, testEmail, td.TestUid)

		assert.Assert(t, is.Nil(sub))
		assertServerErrorContains(t, err, "error getting "+testEmail)
	})
}

func TestVerify(t *testing.T) {
	setup := func() (
		*ProdAgent,
		*testdoubles.Database,
		*db.Subscriber,
		context.Context) {
		f := newProdAgentTestFixture()
		sub := &db.Subscriber{
			Email:     testEmail,
			Uid:       td.TestUid,
			Status:    db.SubscriberPending,
			Timestamp: td.TestTimestamp,
		}
		return f.agent, f.db, sub, context.Background()
	}

	t.Run("SucceedsAndUpdatesStatusAndTimestamp", func(t *testing.T) {
		agent, dbase, pendingSub, ctx := setup()
		newTimestamp := time.Now().Truncate(time.Second)
		agent.CurrentTime = func() time.Time {
			return newTimestamp
		}
		assert.NilError(t, dbase.Put(ctx, pendingSub))

		result, err := agent.Verify(ctx, pendingSub.Email, pendingSub.Uid)

		assert.NilError(t, err)
		assert.Equal(t, ops.Subscribed, result)

		sub, err := dbase.Get(ctx, pendingSub.Email)
		assert.NilError(t, err)
		assert.Equal(t, db.SubscriberVerified, sub.Status)
		assert.Equal(t, newTimestamp, sub.Timestamp)
	})

	t.Run("ReturnsNotSubscribedIfNotFound", func(t *testing.T) {
		agent, _, pendingSub, ctx := setup()

		result, err := agent.Verify(ctx, pendingSub.Email, pendingSub.Uid)

		assert.NilError(t, err)
		assert.Equal(t, ops.NotSubscribed, result)
	})

	t.Run("ReturnsAlreadySubscribedIfAlreadyVerified", func(t *testing.T) {
		agent, dbase, _, ctx := setup()
		verifiedSub := verifiedSubscriber
		assert.NilError(t, dbase.Put(ctx, verifiedSub))

		result, err := agent.Verify(ctx, verifiedSub.Email, verifiedSub.Uid)

		assert.NilError(t, err)
		assert.Equal(t, ops.AlreadySubscribed, result)
	})

	t.Run("PassesThroughGetSubscriberError", func(t *testing.T) {
		agent, dbase, pendingSub, ctx := setup()
		dbase.SimulateGetErr = func(address string) error {
			return makeServerError("failed to get " + address)
		}

		result, err := agent.Verify(ctx, pendingSub.Email, pendingSub.Uid)

		assert.Equal(t, ops.Invalid, result)
		assertServerErrorContains(t, err, "failed to get "+pendingSub.Email)
	})

	t.Run("PassesThroughPutError", func(t *testing.T) {
		agent, dbase, pendingSub, ctx := setup()
		assert.NilError(t, dbase.Put(ctx, pendingSub))
		dbase.SimulatePutErr = func(address string) error {
			return makeServerError("failed to put " + address)
		}

		result, err := agent.Verify(ctx, pendingSub.Email, pendingSub.Uid)

		assert.Equal(t, ops.Invalid, result)
		assertServerErrorContains(t, err, "failed to put "+pendingSub.Email)
	})
}

func TestUnsubscribe(t *testing.T) {
	setup := func() (
		*ProdAgent,
		*testdoubles.Database,
		*db.Subscriber,
		context.Context) {
		f := newProdAgentTestFixture()
		sub := &db.Subscriber{
			Email:     testEmail,
			Uid:       td.TestUid,
			Status:    db.SubscriberVerified,
			Timestamp: td.TestTimestamp,
		}
		return f.agent, f.db, sub, context.Background()
	}

	t.Run("Succeeds", func(t *testing.T) {
		agent, dbase, sub, ctx := setup()
		assert.NilError(t, dbase.Put(ctx, sub))

		result, err := agent.Unsubscribe(ctx, sub.Email, sub.Uid)

		assert.NilError(t, err)
		assert.Equal(t, ops.Unsubscribed, result)
		assert.Assert(t, is.Nil(dbase.Index[sub.Email]))
	})

	t.Run("ReturnsNotSubscribedIfSubscriberNotFound", func(t *testing.T) {
		agent, _, sub, ctx := setup()

		result, err := agent.Unsubscribe(ctx, sub.Email, sub.Uid)

		assert.NilError(t, err)
		assert.Equal(t, ops.NotSubscribed, result)
	})

	t.Run("PassesThroughGetSubscriberError", func(t *testing.T) {
		agent, dbase, sub, ctx := setup()
		dbase.SimulateGetErr = func(address string) error {
			return makeServerError("failed to get " + address)
		}

		result, err := agent.Unsubscribe(ctx, sub.Email, sub.Uid)

		assert.Equal(t, ops.Invalid, result)
		assertServerErrorContains(t, err, "failed to get "+sub.Email)
	})

	t.Run("PassesThroughPutError", func(t *testing.T) {
		agent, dbase, sub, ctx := setup()
		assert.NilError(t, dbase.Put(ctx, sub))
		dbase.SimulateDelErr = func(address string) error {
			return makeServerError("failed to delete " + address)
		}

		result, err := agent.Unsubscribe(ctx, sub.Email, sub.Uid)

		assert.Equal(t, ops.Invalid, result)
		assertServerErrorContains(t, err, "failed to delete "+sub.Email)
	})
}

func TestRemove(t *testing.T) {
	setup := func() (
		*ProdAgent,
		*testdoubles.Database,
		*testdoubles.Suppressor,
		*db.Subscriber,
		context.Context) {
		f := newProdAgentTestFixture()
		sub := &db.Subscriber{
			Email:     testEmail,
			Uid:       td.TestUid,
			Status:    db.SubscriberVerified,
			Timestamp: td.TestTimestamp,
		}
		return f.agent, f.db, f.suppressor, sub, context.Background()
	}

	t.Run("Succeeds", func(t *testing.T) {
		agent, dbase, suppressor, sub, ctx := setup()
		assert.NilError(t, dbase.Put(ctx, sub))

		err := agent.Remove(ctx, sub.Email)

		assert.NilError(t, err)
		assert.Assert(t, is.Nil(dbase.Index[sub.Email]))
		assert.Assert(t, suppressor.Addresses[sub.Email] == true)
	})

	t.Run("PassesThroughDeleteError", func(t *testing.T) {
		agent, dbase, _, sub, ctx := setup()
		dbase.SimulateDelErr = func(address string) error {
			return makeServerError("failed to delete " + address)
		}

		err := agent.Remove(ctx, sub.Email)

		assertServerErrorContains(t, err, "failed to delete "+sub.Email)
	})

	t.Run("PassesThroughSuppressError", func(t *testing.T) {
		agent, _, suppressor, sub, ctx := setup()
		errMsg := "failed to suppress " + sub.Email
		suppressor.Errors[sub.Email] = makeServerError(errMsg)

		err := agent.Remove(ctx, sub.Email)

		assertServerErrorContains(t, err, errMsg)
	})
}

func TestRestore(t *testing.T) {
	setup := func() (
		*ProdAgent,
		*testdoubles.Database,
		*testdoubles.Suppressor,
		*db.Subscriber,
		context.Context) {
		f := newProdAgentTestFixture()
		expectedSub := &db.Subscriber{
			Email:     testEmail,
			Uid:       td.TestUid,
			Status:    db.SubscriberVerified,
			Timestamp: td.TestTimestamp,
		}
		return f.agent, f.db, f.suppressor, expectedSub, context.Background()
	}

	t.Run("Succeeds", func(t *testing.T) {
		agent, dbase, suppressor, expectedSub, ctx := setup()
		suppressor.Addresses[expectedSub.Email] = true

		err := agent.Restore(ctx, expectedSub.Email)

		assert.NilError(t, err)
		assert.DeepEqual(t, expectedSub, dbase.Index[expectedSub.Email])
		assert.Assert(t, suppressor.Addresses[expectedSub.Email] == false)
	})

	t.Run("PassesThroughPutError", func(t *testing.T) {
		agent, dbase, _, expectedSub, ctx := setup()
		dbase.SimulatePutErr = func(address string) error {
			return makeServerError("failed to put " + address)
		}

		err := agent.Restore(ctx, expectedSub.Email)

		assertServerErrorContains(t, err, "failed to put "+expectedSub.Email)
	})

	t.Run("PassesThroughUnsuppressError", func(t *testing.T) {
		agent, _, suppressor, sub, ctx := setup()
		errMsg := "failed to unsuppress " + sub.Email
		suppressor.Errors[sub.Email] = makeServerError(errMsg)

		err := agent.Restore(ctx, sub.Email)

		assertServerErrorContains(t, err, errMsg)
	})
}

func TestSend(t *testing.T) {
	msg := &email.Message{}
	err := json.Unmarshal([]byte(email.ExampleMessageJson), &msg)
	if err != nil {
		panic("email.ExampleMessageJson failed to unmarshal: " + err.Error())
	}
	msg.From = testSender

	setup := func() (
		*ProdAgent,
		*testdoubles.Database,
		*testdoubles.Mailer,
		*tu.Logs,
		context.Context) {
		f := newProdAgentTestFixture()
		ctx := context.Background()

		for i, sub := range db.TestSubscribers {
			if err := f.db.Put(ctx, sub); err != nil {
				msg := "failed to Put test subscriber " + sub.Email + ": " +
					err.Error()
				panic(msg)
			}
			f.mailer.MessageIds[sub.Email] = fmt.Sprintf("msg-%d", i)
		}
		return f.agent, f.db, f.mailer, f.logs, ctx
	}

	assertSentToVerifiedSubscriber := func(
		t *testing.T,
		i int,
		sub *db.Subscriber,
		mailer *testdoubles.Mailer,
		logs *tu.Logs,
	) {
		t.Helper()

		msgId, m := mailer.GetMessageTo(t, sub.Email)
		unsubUrl := ops.UnsubscribeUrl(testUnsubBaseUrl, sub.Email, sub.Uid)
		unsubMailto := ops.UnsubscribeMailto(testUnsubEmail, sub.Email, sub.Uid)
		expectedLogMsg := fmt.Sprintf(
			"sent \"%s\" id: %s to: %s", msg.Subject, msgId, sub.Email,
		)
		assert.Equal(t, mailer.MessageIds[sub.Email], msgId)
		assert.Assert(t, is.Contains(m, unsubUrl))
		assert.Assert(t, is.Contains(m, unsubMailto))
		logs.AssertContains(t, expectedLogMsg)
	}

	assertSentToVerifiedSubscribers := func(
		t *testing.T, mailer *testdoubles.Mailer, logs *tu.Logs,
	) {
		t.Helper()

		for i, sub := range db.TestVerifiedSubscribers {
			assertSentToVerifiedSubscriber(t, i, sub, mailer, logs)
		}
	}

	assertDidNotSendToPendingSubscribers := func(
		t *testing.T, mailer *testdoubles.Mailer,
	) {
		t.Helper()

		for _, sub := range db.TestPendingSubscribers {
			mailer.AssertNoMessageSent(t, sub.Email)
		}
	}

	t.Run("Succeeds", func(t *testing.T) {
		agent, _, mailer, logs, ctx := setup()

		numSent, err := agent.Send(ctx, msg)

		assert.NilError(t, err)
		assertSentToVerifiedSubscribers(t, mailer, logs)
		assertDidNotSendToPendingSubscribers(t, mailer)
		assert.Equal(t, len(db.TestVerifiedSubscribers), numSent)
	})

	t.Run("FailsIfMessageFailsValidation", func(t *testing.T) {
		agent, _, _, _, ctx := setup()
		badMsg := *msg
		badMsg.From = ""

		numSent, err := agent.Send(ctx, &badMsg)

		const expectedErrMsg = "message failed validation: missing From"
		assert.ErrorContains(t, err, expectedErrMsg)
		assert.Equal(t, 0, numSent)
	})

	t.Run("FailsIfNoBulkCapacityAvailable", func(t *testing.T) {
		agent, _, mailer, _, ctx := setup()
		mailer.BulkCapError = email.ErrBulkSendCapacityExhausted

		numSent, err := agent.Send(ctx, msg)

		const expectedErrMsg = "couldn't send to subscribers: "
		assert.ErrorContains(t, err, expectedErrMsg)
		assert.Assert(t, tu.ErrorIs(err, email.ErrBulkSendCapacityExhausted))
		assert.Equal(t, 0, numSent)
	})

	t.Run("FailsIfProcessSubscribersInStateFails", func(t *testing.T) {
		agent, dbase, _, _, ctx := setup()
		procSubsErr := errors.New("ProcSubsInState error")
		dbase.SimulateProcSubsErr = func(_ string) error {
			return procSubsErr
		}

		numSent, err := agent.Send(ctx, msg)

		expectedErrMsg := fmt.Sprintf(
			"error sending \"%s\" to list: ProcSubsInState error", msg.Subject,
		)
		assert.Error(t, err, expectedErrMsg)
		assert.Assert(t, tu.ErrorIs(err, procSubsErr))
		assert.Equal(t, 0, numSent)
	})

	t.Run("StopsProcessingAndFailsIfSendFails", func(t *testing.T) {
		agent, _, mailer, logs, ctx := setup()
		secondSub := db.TestVerifiedSubscribers[1]
		sendErr := errors.New("Mailer.Send failed")
		mailer.RecipientErrors[secondSub.Email] = sendErr

		numSent, err := agent.Send(ctx, msg)

		assert.Assert(t, tu.ErrorIs(err, sendErr))
		assertSentToVerifiedSubscriber(
			t, 0, db.TestVerifiedSubscribers[0], mailer, logs,
		)
		mailer.AssertNoMessageSent(t, secondSub.Email)
		assert.Equal(t, 1, numSent)
	})
}
