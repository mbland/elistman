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
const testUnsubUrl = "https://foo.com/unsubscribe"
const testApiBaseUrl = "https://foo.com/email/"

func testMessage() (msg *email.Message) {
	msg = &email.Message{}
	err := json.Unmarshal([]byte(email.ExampleMessageJson), &msg)
	if err != nil {
		panic("email.ExampleMessageJson failed to unmarshal: " + err.Error())
	}
	msg.From = testSender
	return
}

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
		testUnsubUrl,
		testApiBaseUrl,
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

func (f *prodAgentTestFixture) setupTestSubscribers() {
	ctx := context.Background()
	for i, sub := range db.TestSubscribers {
		if err := f.db.Put(ctx, sub); err != nil {
			msg := "failed to Put test subscriber " + sub.Email + ": " +
				err.Error()
			panic(msg)
		}
		f.mailer.MessageIds[sub.Email] = fmt.Sprintf("msg-%d", i)
	}
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
		agent *ProdAgent, validator *testdoubles.AddressValidator,
	) {
		f := newProdAgentTestFixture()
		return f.agent, f.validator
	}

	ctx := context.Background()

	t.Run("Succeeds", func(t *testing.T) {
		agent, validator := setup()

		failure, err := agent.Validate(ctx, testEmail)

		assert.NilError(t, err)
		assert.Assert(t, is.Nil(failure))
		validator.AssertValidated(t, testEmail)
	})

	t.Run("FailsIfValidationFails", func(t *testing.T) {
		agent, validator := setup()
		validator.Failure = &email.ValidationFailure{
			Address: testEmail, Reason: "test failure",
		}

		failure, err := agent.Validate(ctx, testEmail)

		assert.NilError(t, err)
		assert.Equal(t, validator.Failure, failure)
		validator.AssertValidated(t, testEmail)
	})

	t.Run("PassesThroughError", func(t *testing.T) {
		agent, validator := setup()
		validator.Error = makeServerError("test error")

		failure, err := agent.Validate(ctx, testEmail)

		assert.Assert(t, is.Nil(failure))
		validator.AssertValidated(t, testEmail)
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

func TestImport(t *testing.T) {
	setup := func() (
		agent *ProdAgent,
		validator *testdoubles.AddressValidator,
		dbase *testdoubles.Database,
		subscriber *db.Subscriber,
	) {
		f := newProdAgentTestFixture()
		agent = f.agent
		validator = f.validator
		dbase = f.db
		uid, _ := agent.NewUid()
		subscriber = &db.Subscriber{
			Email:     testEmail,
			Uid:       uid,
			Status:    db.SubscriberVerified,
			Timestamp: agent.CurrentTime(),
		}
		return
	}

	ctx := context.Background()

	t.Run("Succeeds", func(t *testing.T) {
		agent, validator, dbase, expectedSubscriber := setup()

		err := agent.Import(ctx, testEmail)

		assert.NilError(t, err)
		validator.AssertValidated(t, testEmail)
		assert.DeepEqual(t, expectedSubscriber, dbase.Index[testEmail])
	})

	t.Run("OverwritesExistingPendingSubscriber", func(t *testing.T) {
		agent, validator, dbase, expectedSubscriber := setup()
		dbase.Put(ctx, pendingSubscriber)

		err := agent.Import(ctx, testEmail)

		assert.NilError(t, err)
		validator.AssertValidated(t, testEmail)
		assert.DeepEqual(t, expectedSubscriber, dbase.Index[testEmail])
	})

	t.Run("ReturnsValidationError", func(t *testing.T) {
		agent, validator, dbase, _ := setup()
		validator.Failure = &email.ValidationFailure{
			Address: testEmail, Reason: "test failure",
		}

		err := agent.Import(ctx, testEmail)

		validator.AssertValidated(t, testEmail)
		assert.ErrorContains(t, err, validator.Failure.Reason)
		assert.Assert(t, is.Nil(dbase.Index[testEmail]))
	})

	t.Run("ReportsValidationFailureAsError", func(t *testing.T) {
		agent, validator, dbase, _ := setup()
		validator.Error = makeServerError("test error")

		err := agent.Import(ctx, testEmail)

		validator.AssertValidated(t, testEmail)
		assertServerErrorContains(t, err, "test error")
		assert.Assert(t, is.Nil(dbase.Index[testEmail]))
	})

	t.Run("ReturnsErrorIfVerifiedSubscriberAlreadyExists", func(t *testing.T) {
		agent, validator, dbase, _ := setup()
		// verifiedSubscriber.UUID is different from that of a new subscriber.
		dbase.Put(ctx, verifiedSubscriber)

		err := agent.Import(ctx, testEmail)

		assert.ErrorContains(t, err, "already a verified subscriber")
		validator.AssertValidated(t, testEmail)
		assert.DeepEqual(t, verifiedSubscriber, dbase.Index[testEmail])
	})

	t.Run("PassesThroughDatabaseGetError", func(t *testing.T) {
		agent, validator, dbase, _ := setup()
		dbase.SimulateGetErr = func(_ string) error {
			return makeServerError("test error")
		}

		err := agent.Import(ctx, testEmail)

		validator.AssertValidated(t, testEmail)
		assertServerErrorContains(t, err, "test error")
	})

	t.Run("PassesThroughDatabasePutError", func(t *testing.T) {
		agent, validator, dbase, _ := setup()
		dbase.SimulatePutErr = func(_ string) error {
			return makeServerError("test error")
		}

		err := agent.Import(ctx, testEmail)

		validator.AssertValidated(t, testEmail)
		assertServerErrorContains(t, err, "test error")
		assert.Assert(t, is.Nil(dbase.Index[testEmail]))
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
		reason := ops.RemoveReasonComplaint
		assert.NilError(t, dbase.Put(ctx, sub))

		err := agent.Remove(ctx, sub.Email, reason)

		assert.NilError(t, err)
		assert.Assert(t, is.Nil(dbase.Index[sub.Email]))
		assert.Equal(t, reason, suppressor.Addresses[sub.Email])
	})

	t.Run("PassesThroughDeleteError", func(t *testing.T) {
		agent, dbase, _, sub, ctx := setup()
		dbase.SimulateDelErr = func(address string) error {
			return makeServerError("failed to delete " + address)
		}

		err := agent.Remove(ctx, sub.Email, ops.RemoveReasonComplaint)

		assertServerErrorContains(t, err, "failed to delete "+sub.Email)
	})

	t.Run("PassesThroughSuppressError", func(t *testing.T) {
		agent, _, suppressor, sub, ctx := setup()
		errMsg := "failed to suppress " + sub.Email
		suppressor.Errors[sub.Email] = makeServerError(errMsg)

		err := agent.Remove(ctx, sub.Email, ops.RemoveReasonComplaint)

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
		suppressor.Addresses[expectedSub.Email] = ops.RemoveReasonComplaint

		err := agent.Restore(ctx, expectedSub.Email)

		assert.NilError(t, err)
		assert.DeepEqual(t, expectedSub, dbase.Index[expectedSub.Email])
		assert.Equal(
			t, ops.RemoveReasonNil, suppressor.Addresses[expectedSub.Email],
		)
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

func assertSentToVerifiedSubscriber(
	t *testing.T,
	subject string,
	sub *db.Subscriber,
	mailer *testdoubles.Mailer,
	logs *tu.Logs,
) {
	t.Helper()

	msgId, m := mailer.GetMessageTo(t, sub.Email)
	unsubUrl := ops.UnsubscribeUrl(testApiBaseUrl, sub.Email, sub.Uid)
	unsubMailto := ops.UnsubscribeMailto(testUnsubEmail, sub.Email, sub.Uid)
	expectedLogMsg := fmt.Sprintf(
		"sent \"%s\" id: %s to: %s", subject, msgId, sub.Email,
	)
	assert.Assert(t, is.Contains(m, unsubUrl))
	assert.Assert(t, is.Contains(m, unsubMailto))
	logs.AssertContains(t, expectedLogMsg)
}

func assertSentToVerifiedSubscribers(
	t *testing.T, subject string, mailer *testdoubles.Mailer, logs *tu.Logs,
) {
	t.Helper()

	for _, sub := range db.TestVerifiedSubscribers {
		assertSentToVerifiedSubscriber(t, subject, sub, mailer, logs)
	}
}

func assertDidNotSendToPendingSubscribers(
	t *testing.T, mailer *testdoubles.Mailer,
) {
	t.Helper()

	for _, sub := range db.TestPendingSubscribers {
		mailer.AssertNoMessageSent(t, sub.Email)
	}
}

func TestSend(t *testing.T) {
	setup := func() (
		*ProdAgent,
		*testdoubles.Database,
		*testdoubles.Mailer,
		*tu.Logs,
		context.Context) {
		f := newProdAgentTestFixture()
		ctx := context.Background()

		f.setupTestSubscribers()
		return f.agent, f.db, f.mailer, f.logs, ctx
	}

	msg := testMessage()
	subject := msg.Subject

	getAddrs := func(subs ...*db.Subscriber) (addrs []string) {
		addrs = make([]string, len(subs))
		for i := range subs {
			addrs[i] = subs[i].Email
		}
		return
	}

	t.Run("ToEntireList", func(t *testing.T) {
		t.Run("Succeeds", func(t *testing.T) {
			agent, _, mailer, logs, ctx := setup()

			numSent, err := agent.Send(ctx, msg, []string{})

			assert.NilError(t, err)
			assertSentToVerifiedSubscribers(t, subject, mailer, logs)
			assertDidNotSendToPendingSubscribers(t, mailer)
			assert.Equal(t, len(db.TestVerifiedSubscribers), numSent)
		})

		t.Run("FailsIfNoBulkCapacityAvailable", func(t *testing.T) {
			agent, _, mailer, _, ctx := setup()
			mailer.BulkCapError = email.ErrBulkSendCapacityExhausted

			numSent, err := agent.Send(ctx, msg, []string{})

			const expectedErrMsg = "couldn't send to subscribers: "
			assert.ErrorContains(t, err, expectedErrMsg)
			assert.Assert(
				t, tu.ErrorIs(err, email.ErrBulkSendCapacityExhausted),
			)
			assert.Equal(t, 0, numSent)
		})

		t.Run("FailsIfProcessSubscribersInStateFails", func(t *testing.T) {
			agent, dbase, _, _, ctx := setup()
			procSubsErr := errors.New("ProcSubsInState error")
			dbase.SimulateProcSubsErr = func(_ string) error {
				return procSubsErr
			}

			numSent, err := agent.Send(ctx, msg, []string{})

			expectedErrMsg := fmt.Sprintf(
				"error sending \"%s\" to list: ProcSubsInState error", subject,
			)
			assert.Error(t, err, expectedErrMsg)
			assert.Assert(t, tu.ErrorIs(err, procSubsErr))
			assert.Equal(t, 0, numSent)
		})

		t.Run("StopsProcessingAndFailsIfSendOneEmailFails", func(t *testing.T) {
			agent, _, mailer, logs, ctx := setup()
			subs := []*db.Subscriber{
				db.TestVerifiedSubscribers[0], db.TestVerifiedSubscribers[1],
			}
			sendErr := errors.New("Mailer.Send failed")
			mailer.RecipientErrors[subs[1].Email] = sendErr

			numSent, err := agent.Send(ctx, msg, []string{})

			assert.Assert(t, tu.ErrorIs(err, sendErr))
			assertSentToVerifiedSubscriber(t, subject, subs[0], mailer, logs)
			mailer.AssertNoMessageSent(t, subs[1].Email)
			assert.Equal(t, 1, numSent)
		})
	})

	t.Run("ToSpecificRecipients", func(t *testing.T) {
		t.Run("Succeeds", func(t *testing.T) {
			agent, _, mailer, logs, ctx := setup()
			subs := []*db.Subscriber{
				db.TestVerifiedSubscribers[0], db.TestVerifiedSubscribers[2],
			}
			addrs := getAddrs(subs...)

			numSent, err := agent.Send(ctx, msg, addrs)

			assert.NilError(t, err)
			assert.Equal(t, len(addrs), numSent)
			assertSentToVerifiedSubscriber(t, subject, subs[0], mailer, logs)
			assertSentToVerifiedSubscriber(t, subject, subs[1], mailer, logs)
		})

		t.Run("FailsIfDbGetReturnsError", func(t *testing.T) {
			agent, dbase, mailer, logs, ctx := setup()
			subs := []*db.Subscriber{
				db.TestVerifiedSubscribers[0], db.TestVerifiedSubscribers[2],
			}
			addrs := getAddrs(subs...)
			getErr := errors.New("Get error")
			dbase.SimulateGetErr = func(addr string) error {
				// Return an error on the first address, but the second should still
				// succeed.
				if addr == addrs[0] {
					return getErr
				}
				return nil
			}

			numSent, err := agent.Send(ctx, msg, addrs)

			assert.Equal(t, 1, numSent)
			assert.Assert(t, tu.ErrorIs(err, getErr))
			assertSentToVerifiedSubscriber(t, subject, subs[1], mailer, logs)
			mailer.AssertNoMessageSent(t, addrs[0])
		})

		t.Run("FailsIfAddressNotVerified", func(t *testing.T) {
			agent, _, mailer, _, ctx := setup()
			addr := db.TestPendingSubscribers[0].Email

			numSent, err := agent.Send(ctx, msg, []string{addr})

			assert.Equal(t, 0, numSent)
			assert.ErrorContains(t, err, addr+": not verified")
			mailer.AssertNoMessageSent(t, addr)
		})

		t.Run("FailsIfSendOneEmailFails", func(t *testing.T) {
			agent, _, mailer, _, ctx := setup()
			addr := db.TestVerifiedSubscribers[0].Email
			sendErr := errors.New("Mailer.Send failed")
			mailer.RecipientErrors[addr] = sendErr

			numSent, err := agent.Send(ctx, msg, []string{addr})

			assert.Equal(t, 0, numSent)
			assert.Assert(t, tu.ErrorIs(err, sendErr))
			mailer.AssertNoMessageSent(t, addr)
		})
	})

	t.Run("FailsIfMessageFailsValidationDueToFromDomain", func(t *testing.T) {
		agent, _, _, _, ctx := setup()
		badMsg := *msg
		badMsg.From = "Blog Updates <updates@bar.com>"

		numSent, err := agent.Send(ctx, &badMsg, []string{})

		const expectedErr = "domain of From address is not " + testDomainName
		assert.ErrorContains(t, err, expectedErr)
		assert.Equal(t, 0, numSent)
	})
}
