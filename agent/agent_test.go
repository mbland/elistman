//go:build small_tests || all_tests

package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testdoubles"
	tu "github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

const testEmail = "foo@bar.com"
const testSender = "updates@foo.com"
const testSiteTitle = "Foo Blog"
const testUnsubEmail = "unsubscribe@foo.com"
const testUnsubBaseUrl = "https://foo.com/email/"

var expectedSubscriber *db.Subscriber = &db.Subscriber{
	Email:     testEmail,
	Uid:       tu.TestUid,
	Status:    db.SubscriberPending,
	Timestamp: tu.TestTimestamp,
}

var verifiedSubscriber *db.Subscriber = &db.Subscriber{
	Email:     testEmail,
	Uid:       uuid.MustParse("55555555-6666-7777-8888-999999999999"),
	Status:    db.SubscriberVerified,
	Timestamp: time.Now(),
}

type prodAgentTestFixture struct {
	agent     *ProdAgent
	db        *testdoubles.Database
	validator *testdoubles.AddressValidator
	logs      *tu.Logs
}

func newProdAgentTestFixture() *prodAgentTestFixture {
	newUid := func() (uuid.UUID, error) {
		return tu.TestUid, nil
	}
	currentTime := func() time.Time {
		return tu.TestTimestamp
	}
	db := testdoubles.NewDatabase()
	av := testdoubles.NewAddressValidator()
	logs, logger := tu.NewLogs()
	pa := &ProdAgent{
		testSender,
		testSiteTitle,
		testUnsubEmail,
		testUnsubBaseUrl,
		newUid,
		currentTime,
		db,
		av,
		nil,
		logger,
	}
	return &prodAgentTestFixture{pa, db, av, logs}
}

func TestGetOrCreateSubscriber(t *testing.T) {
	setup := func() (*prodAgentTestFixture, context.Context) {
		return newProdAgentTestFixture(), context.Background()
	}

	t.Run("CreatesNewSubscriber", func(t *testing.T) {
		f, ctx := setup()

		sub, err := f.agent.getOrCreateSubscriber(ctx, testEmail)

		assert.NilError(t, err)
		assert.DeepEqual(t, expectedSubscriber, sub)
		assert.DeepEqual(t, expectedSubscriber, f.db.Index[testEmail])
	})

	t.Run("ReturnsVerifiedSubscriberUnchanged", func(t *testing.T) {
		f, ctx := setup()
		assert.NilError(t, f.db.Put(ctx, verifiedSubscriber))

		sub, err := f.agent.getOrCreateSubscriber(ctx, testEmail)

		assert.NilError(t, err)
		assert.DeepEqual(t, verifiedSubscriber, sub)
		assert.DeepEqual(t, verifiedSubscriber, f.db.Index[testEmail])
	})

	t.Run("ReturnsPendingSubscriberWithNewUidAndTimestamp", func(t *testing.T) {
		f, ctx := setup()
		pendingSubscriber := &db.Subscriber{
			Email:     testEmail,
			Uid:       uuid.MustParse("11111111-2222-3333-5555-888888888888"),
			Status:    db.SubscriberPending,
			Timestamp: time.Now(),
		}
		assert.NilError(t, f.db.Put(ctx, pendingSubscriber))

		sub, err := f.agent.getOrCreateSubscriber(ctx, testEmail)

		assert.NilError(t, err)
		assert.DeepEqual(t, expectedSubscriber, sub)
		assert.DeepEqual(t, expectedSubscriber, f.db.Index[testEmail])
	})

	t.Run("ReturnsErrorIfDatabaseGetFails", func(t *testing.T) {
		f, ctx := setup()
		f.db.SimulateGetErr = func(email string) error {
			return errors.New("test error while getting " + email)
		}

		sub, err := f.agent.getOrCreateSubscriber(ctx, testEmail)

		assert.Assert(t, is.Nil(sub))
		assert.Error(t, err, "test error while getting "+testEmail)
	})

	t.Run("ReturnsErrorIfNewUidFails", func(t *testing.T) {
		f, ctx := setup()
		f.agent.NewUid = func() (uuid.UUID, error) {
			return uuid.Nil, errors.New("NewUid failed")
		}

		sub, err := f.agent.getOrCreateSubscriber(ctx, testEmail)

		assert.Assert(t, is.Nil(sub))
		assert.Error(t, err, "NewUid failed")
	})

	t.Run("ReturnsErrorIfDatabasePutFails", func(t *testing.T) {
		f, ctx := setup()
		f.db.SimulatePutErr = func(email string) error {
			return errors.New("test error while putting " + email)
		}

		sub, err := f.agent.getOrCreateSubscriber(ctx, testEmail)

		assert.Assert(t, is.Nil(sub))
		assert.Error(t, err, "test error while putting "+testEmail)
	})
}

func TestMakeVerificationEmail(t *testing.T) {
	setup := func() *ProdAgent {
		f := newProdAgentTestFixture()
		return f.agent
	}

	sub := &db.Subscriber{
		Email:     tu.TestEmail,
		Uid:       tu.TestUid,
		Status:    db.SubscriberPending,
		Timestamp: tu.TestTimestamp,
	}

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
		assert.Assert(t, is.Contains(htmlPart, verifyLink))
	})
}

func TestSubscribe(t *testing.T) {
	setup := func() (*prodAgentTestFixture, context.Context) {
		return newProdAgentTestFixture(), context.Background()
	}

	t.Run("CreatesNewSubscriber", func(t *testing.T) {
		f, ctx := setup()

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.NilError(t, err)
		assert.Equal(t, ops.VerifyLinkSent, result)
		f.validator.AssertValidated(t, testEmail)
		assert.DeepEqual(t, expectedSubscriber, f.db.Index[testEmail])
	})

	t.Run("DoesNotSendEmailToVerifiedSubscriber", func(t *testing.T) {
		f, ctx := setup()
		assert.NilError(t, f.db.Put(ctx, verifiedSubscriber))

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.NilError(t, err)
		assert.Equal(t, ops.AlreadySubscribed, result)
	})

	t.Run("ReturnsInvalidIfAddressFailsValidation", func(t *testing.T) {
		f, ctx := setup()
		f.validator.Failure = &email.ValidationFailure{Reason: "testing"}

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.NilError(t, err)
		assert.Equal(t, ops.Invalid, result)
		f.logs.AssertContains(t, testEmail+" failed validation: testing")
	})

	t.Run("ReturnsErrorIfValidateAddressReturnsError", func(t *testing.T) {
		f, ctx := setup()
		f.validator.Error = errors.New("unexpected SES error")

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.Error(t, err, "unexpected SES error")
		assert.Equal(t, ops.Invalid, result)
	})

	t.Run("ReturnsErrorIfGetOrCreateSubscriberFails", func(t *testing.T) {
		f, ctx := setup()
		f.db.SimulateGetErr = func(email string) error {
			return errors.New("test error while getting " + email)
		}

		result, err := f.agent.Subscribe(ctx, testEmail)

		assert.Equal(t, ops.Invalid, result)
		assert.Error(t, err, "test error while getting "+testEmail)
	})
}
