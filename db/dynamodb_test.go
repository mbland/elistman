//go:build small_tests || all_tests

package db

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestGetAttribute(t *testing.T) {
	testEmail := "mbland@acm.org"
	attrs := dbAttributes{
		"email":      &dbString{Value: testEmail},
		"unexpected": &dbBool{Value: false},
	}

	parseString := func(attr *dbString) (string, error) {
		return attr.Value, nil
	}

	t.Run("Succeeds", func(t *testing.T) {
		value, err := getAttribute("email", attrs, parseString)

		assert.NilError(t, err)
		assert.Equal(t, testEmail, value)
	})

	t.Run("ErrorsIfAttributeNotPresent", func(t *testing.T) {
		value, err := getAttribute("missing", attrs, parseString)

		assert.Equal(t, "", value)
		assert.ErrorContains(t, err, "attribute 'missing' not in: ")
	})

	t.Run("ErrorsIfNotExpectedAttributeType", func(t *testing.T) {
		value, err := getAttribute("unexpected", attrs, parseString)

		assert.Equal(t, "", value)
		errMsg := "attribute 'unexpected' is of type " +
			"*types.AttributeValueMemberBOOL, not "
		assert.ErrorContains(t, err, errMsg)
	})

	t.Run("ErrorsIfParsingFails", func(t *testing.T) {
		parseFail := func(attr *dbString) (string, error) {
			return "shouldn't see this", errors.New("parse failure")
		}

		value, err := getAttribute("email", attrs, parseFail)

		assert.Equal(t, "", value)
		assert.ErrorContains(t, err, "failed to parse 'email' from: ")
		assert.ErrorContains(t, err, ": parse failure")
	})
}

func TestParseSubscriber(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		testEmail := "mbland@acm.org"
		testUid := uuid.New()
		testTime := time.Now().Truncate(time.Second)
		attrs := dbAttributes{
			"email":     &dbString{Value: testEmail},
			"uid":       &dbString{Value: testUid.String()},
			"verified":  &dbBool{Value: true},
			"timestamp": &dbString{Value: testTime.Format(timeFmt)},
		}

		subscriber, err := ParseSubscriber(attrs)

		assert.NilError(t, err)
		assert.DeepEqual(t, subscriber, &Subscriber{
			testEmail, testUid, true, testTime,
		})
	})

	t.Run("ErrorsIfGettingAttributesFail", func(t *testing.T) {
		subscriber, err := ParseSubscriber(dbAttributes{})

		assert.Assert(t, is.Nil(subscriber))
		assert.ErrorContains(t, err, "failed to parse subscriber: ")
		assert.ErrorContains(t, err, "attribute 'email' not in: ")
		assert.ErrorContains(t, err, "attribute 'uid' not in: ")
		assert.ErrorContains(t, err, "attribute 'verified' not in: ")
		assert.ErrorContains(t, err, "attribute 'timestamp' not in: ")
	})
}
