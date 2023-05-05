//go:build small_tests || all_tests

package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

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
		attrs := dbAttributes{
			"email":    &dbString{Value: testEmail},
			"uid":      &dbString{Value: testUid.String()},
			"verified": toDynamoDbTimestamp(testTimestamp),
		}

		subscriber, err := ParseSubscriber(attrs)

		assert.NilError(t, err)
		assert.DeepEqual(t, subscriber, &Subscriber{
			testEmail, testUid, SubscriberVerified, testTimestamp,
		})
	})

	t.Run("ErrorsIfGettingAttributesFail", func(t *testing.T) {
		subscriber, err := ParseSubscriber(dbAttributes{})

		assert.Check(t, is.Nil(subscriber))
		assert.ErrorContains(t, err, "failed to parse subscriber: ")
		assert.ErrorContains(t, err, "attribute 'email' not in: ")
		assert.ErrorContains(t, err, "attribute 'uid' not in: ")

		const errFmt = "has neither '%s' or '%s' attributes"
		expected := fmt.Sprintf(
			errFmt, SubscriberPending, SubscriberVerified,
		)
		assert.ErrorContains(t, err, expected)
	})

	t.Run("ErrorsIfContainsBothPendingAndVerified", func(t *testing.T) {
		attrs := dbAttributes{
			"email":    &dbString{Value: "foo@bar.com"},
			"uid":      &dbString{Value: testUid.String()},
			"pending":  toDynamoDbTimestamp(testTimestamp),
			"verified": toDynamoDbTimestamp(testTimestamp),
		}

		subscriber, err := ParseSubscriber(attrs)

		assert.Check(t, is.Nil(subscriber))

		const errFmt = "contains both '%s' and '%s' attributes"
		expected := fmt.Sprintf(
			errFmt, SubscriberPending, SubscriberVerified,
		)
		assert.ErrorContains(t, err, expected)
	})

	t.Run("ErrorsIfTimestampIsNotAnInteger", func(t *testing.T) {
		attrs := dbAttributes{
			"email":    &dbString{Value: testEmail},
			"uid":      &dbString{Value: testUid.String()},
			"verified": &dbNumber{Value: "not an int"},
		}

		subscriber, err := ParseSubscriber(attrs)

		assert.Check(t, is.Nil(subscriber))
		assert.ErrorContains(t, err, "failed to parse 'verified' from: ")
	})
}

const testStartKeyValue = "foo@bar.com"

var testStartKeyAttrs dbAttributes = dbAttributes{
	"primary": &dbString{Value: testStartKeyValue},
}
var testStartKey *dynamoDbStartKey = &dynamoDbStartKey{testStartKeyAttrs}

type BogusDbStartKey struct{}

func (*BogusDbStartKey) isDbStartKey() {}

func TestNewScanInput(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		t.Run("WithNilStartKey", func(t *testing.T) {
			input, err := newScanInput(
				"subscribers", SubscriberVerified, nil,
			)

			assert.NilError(t, err)
			assert.Equal(t, "subscribers", *input.TableName)
			assert.Equal(t, DynamoDbVerifiedIndexName, *input.IndexName)
			assert.Assert(t, is.Nil(input.ExclusiveStartKey))
		})

		t.Run("WithExistingStartKey", func(t *testing.T) {
			input, err := newScanInput(
				"subscribers", SubscriberPending, testStartKey,
			)

			assert.NilError(t, err)
			assert.Equal(t, "subscribers", *input.TableName)
			assert.Equal(t, DynamoDbPendingIndexName, *input.IndexName)
			assert.Assert(t, is.Contains(input.ExclusiveStartKey, "primary"))

			actualKey := input.ExclusiveStartKey["primary"].(*dbString)
			assert.Equal(t, testStartKeyValue, actualKey.Value)
		})
	})

	t.Run("ErrorsIfInvalidStartKey", func(t *testing.T) {
		input, err := newScanInput(
			"subscribers", SubscriberVerified, &BogusDbStartKey{},
		)

		assert.Assert(t, is.Nil(input))
		assert.Error(t, err, "not a *db.dynamoDbStartKey: *db.BogusDbStartKey")
	})
}

func newSubscriberRecord(sub *Subscriber) dbAttributes {
	return dbAttributes{
		"email":            &dbString{Value: sub.Email},
		"uid":              &dbString{Value: sub.Uid.String()},
		string(sub.Status): toDynamoDbTimestamp(sub.Timestamp),
	}
}

var testVerifiedSubscribers []*Subscriber = []*Subscriber{
	{"foo@test.com", testUid, SubscriberVerified, testTimestamp},
	{"bar@test.com", testUid, SubscriberVerified, testTimestamp},
	{"baz@test.com", testUid, SubscriberVerified, testTimestamp},
}

func TestProcessScanOutput(t *testing.T) {
	setup := func() *dynamodb.ScanOutput {
		return &dynamodb.ScanOutput{
			LastEvaluatedKey: testStartKey.attrs,
			Items: []dbAttributes{
				newSubscriberRecord(testVerifiedSubscribers[0]),
				newSubscriberRecord(testVerifiedSubscribers[1]),
				newSubscriberRecord(testVerifiedSubscribers[2]),
			},
		}
	}
	t.Run("Succeeds", func(t *testing.T) {
		output := setup()

		subs, nextStartKey, err := processScanOutput(output)

		assert.NilError(t, err)

		dbStartKey, ok := nextStartKey.(*dynamoDbStartKey)
		if !ok {
			t.Fatalf("start key is not *dynamoDbStartKey: %T", nextStartKey)
		}
		assert.Assert(t, is.Contains(dbStartKey.attrs, "primary"))
		actualKey := dbStartKey.attrs["primary"].(*dbString)
		assert.Equal(t, testStartKeyValue, actualKey.Value)

		expectedSubs := []*Subscriber{
			testVerifiedSubscribers[0],
			testVerifiedSubscribers[1],
			testVerifiedSubscribers[2],
		}
		assert.DeepEqual(t, expectedSubs, subs)
	})

	t.Run("ReturnsParseSubscriberErrors", func(t *testing.T) {
		output := setup()
		const statusKey string = string(SubscriberPending)
		for _, record := range output.Items {
			record[statusKey] = toDynamoDbTimestamp(testTimestamp)
		}

		subs, _, err := processScanOutput(output)

		assert.DeepEqual(t, []*Subscriber{nil, nil, nil}, subs)
		expectedErr := fmt.Sprintf(
			"failed to parse subscriber: "+
				"contains both '%s' and '%s' attributes",
			SubscriberPending,
			SubscriberVerified,
		)
		assert.ErrorContains(t, err, expectedErr)
	})
}
