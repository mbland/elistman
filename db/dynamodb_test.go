//go:build small_tests || all_tests

package db

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

const testEmail = "foo@bar.com"
const testTimeStr = "Fri, 18 Sep 1970 12:45:00 +0000"

var testUid uuid.UUID = uuid.MustParse("00000000-1111-2222-3333-444444444444")

var testTimestamp time.Time

func init() {
	var err error
	testTimestamp, err = time.Parse(time.RFC1123Z, testTimeStr)

	if err != nil {
		panic("failed to parse testTimestamp: " + err.Error())
	}
}

func TestGetAttribute(t *testing.T) {
	attrs := dbAttributes{
		"email":      &dbString{Value: testEmail},
		"unexpected": &types.AttributeValueMemberBOOL{Value: false},
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
		testTime := time.Now().Truncate(time.Second)
		attrs := dbAttributes{
			"email":     &dbString{Value: testEmail},
			"uid":       &dbString{Value: testUid.String()},
			"verified":  &dbString{Value: "Y"},
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

		assert.Check(t, is.Nil(subscriber))
		assert.ErrorContains(t, err, "failed to parse subscriber: ")
		assert.ErrorContains(t, err, "attribute 'email' not in: ")
		assert.ErrorContains(t, err, "attribute 'uid' not in: ")
		assert.ErrorContains(t, err, "attribute 'timestamp' not in: ")

		const errFmt = "has neither '%s' or '%s' attributes"
		expected := fmt.Sprintf(
			errFmt, SubscriberStatePending, SubscriberStateVerified,
		)
		assert.ErrorContains(t, err, expected)
	})

	t.Run("ErrorsIfContainsBothPendingAndVerified", func(t *testing.T) {
		attrs := dbAttributes{
			"email":     &dbString{Value: "foo@bar.com"},
			"uid":       &dbString{Value: testUid.String()},
			"pending":   &dbString{Value: "Y"},
			"verified":  &dbString{Value: "Y"},
			"timestamp": &dbString{Value: testTimestamp.Format(timeFmt)},
		}

		subscriber, err := ParseSubscriber(attrs)
		assert.Check(t, is.Nil(subscriber))

		const errFmt = "contains both '%s' and '%s' attributes"
		expected := fmt.Sprintf(
			errFmt, SubscriberStatePending, SubscriberStateVerified,
		)
		assert.ErrorContains(t, err, expected)
	})
}
