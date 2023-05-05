//go:build small_tests || all_tests

package db

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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
				"subscribers", SubscriberStateVerified, nil,
			)

			assert.NilError(t, err)
			assert.Equal(t, "subscribers", *input.TableName)
			assert.Equal(t, DynamoDbVerifiedIndexName, *input.IndexName)
			assert.Assert(t, is.Nil(input.ExclusiveStartKey))
		})

		t.Run("WithExistingStartKey", func(t *testing.T) {
			input, err := newScanInput(
				"subscribers", SubscriberStatePending, testStartKey,
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
			"subscribers", SubscriberStateVerified, &BogusDbStartKey{},
		)

		assert.Assert(t, is.Nil(input))
		assert.Error(t, err, "not a *db.dynamoDbStartKey: *db.BogusDbStartKey")
	})
}

func newSubscriberRecord(sub *Subscriber) dbAttributes {
	record := dbAttributes{
		"email":     &dbString{Value: sub.Email},
		"uid":       &dbString{Value: sub.Uid.String()},
		"timestamp": &dbString{Value: sub.Timestamp.Format(timeFmt)},
	}
	var state SubscriberState

	if sub.Verified {
		state = SubscriberStateVerified
	} else {
		state = SubscriberStatePending
	}
	record[string(state)] = &dbString{Value: "Y"}
	return record
}

var testPendingSubscribers []*Subscriber = []*Subscriber{
	{"quux@test.com", testUid, false, testTimestamp},
	{"xyzzy@test.com", testUid, false, testTimestamp},
	{"plugh@test.com", testUid, false, testTimestamp},
}

var testVerifiedSubscribers []*Subscriber = []*Subscriber{
	{"foo@test.com", testUid, true, testTimestamp},
	{"bar@test.com", testUid, true, testTimestamp},
	{"baz@test.com", testUid, true, testTimestamp},
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
		for _, record := range output.Items {
			record[string(SubscriberStatePending)] = &dbString{Value: "Y"}
		}

		subs, _, err := processScanOutput(output)

		assert.DeepEqual(t, []*Subscriber{nil, nil, nil}, subs)
		expectedErr := fmt.Sprintf(
			"failed to parse subscriber: "+
				"contains both '%s' and '%s' attributes",
			SubscriberStatePending,
			SubscriberStateVerified,
		)
		assert.ErrorContains(t, err, expectedErr)
	})
}
