//go:build small_tests || all_tests

package db

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testdata"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

// Most of the methods on TestDynamoDbClient are unimplemented, because
// dynamodb_contract_test.go tests most of them.
//
// The exception to this is Scan(), which is the reason why the DynamoDbClient
// interface exists. Testing all the cases of the code that relies on Scan() is
// annoying, difficult, and/or nearly impossible without using this test double.
type TestDynamoDbClient struct {
	subscribers []dbAttributes
	scanSize    int
	scanCalls   int
	serverErr   error
}

func (client *TestDynamoDbClient) SetServerError(msg string) {
	client.serverErr = testutils.AwsServerError(msg)
}

func (client *TestDynamoDbClient) CreateTable(
	context.Context, *dynamodb.CreateTableInput, ...func(*dynamodb.Options),
) (_ *dynamodb.CreateTableOutput, err error) {
	err = client.serverErr
	return
}

func (client *TestDynamoDbClient) DescribeTable(
	context.Context,
	*dynamodb.DescribeTableInput,
	...func(*dynamodb.Options),
) (_ *dynamodb.DescribeTableOutput, _ error) {
	return nil, client.serverErr
}

func (client *TestDynamoDbClient) UpdateTimeToLive(
	context.Context,
	*dynamodb.UpdateTimeToLiveInput,
	...func(*dynamodb.Options),
) (_ *dynamodb.UpdateTimeToLiveOutput, _ error) {
	return nil, client.serverErr
}

func (client *TestDynamoDbClient) DeleteTable(
	context.Context, *dynamodb.DeleteTableInput, ...func(*dynamodb.Options),
) (_ *dynamodb.DeleteTableOutput, _ error) {
	return nil, client.serverErr
}

func (client *TestDynamoDbClient) GetItem(
	context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options),
) (_ *dynamodb.GetItemOutput, _ error) {
	return nil, client.serverErr
}

func (client *TestDynamoDbClient) PutItem(
	context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options),
) (_ *dynamodb.PutItemOutput, _ error) {
	return nil, client.serverErr
}

func (client *TestDynamoDbClient) DeleteItem(
	context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options),
) (_ *dynamodb.DeleteItemOutput, _ error) {
	return nil, client.serverErr
}

func (client *TestDynamoDbClient) addSubscriberRecord(sub dbAttributes) {
	client.subscribers = append(client.subscribers, sub)
}

func (client *TestDynamoDbClient) addSubscribers(subs []*Subscriber) {
	for _, sub := range subs {
		subRec := newSubscriberRecord(sub)
		client.subscribers = append(client.subscribers, subRec)
	}
}

func (client *TestDynamoDbClient) Scan(
	_ context.Context, input *dynamodb.ScanInput, _ ...func(*dynamodb.Options),
) (output *dynamodb.ScanOutput, err error) {
	client.scanCalls++

	err = client.serverErr
	if err != nil {
		return
	}

	items := make([]dbAttributes, 0, len(client.subscribers))

	// Remember that our schema is to keep pending and verified subscribers
	// partitioned across disjoint Global Secondary Indexes.
	for _, sub := range client.subscribers {
		if _, ok := sub[aws.ToString(input.IndexName)]; ok {
			items = append(items, sub)
		}
	}

	// Simulating pagination is a little tricky. We use the following functions
	// to trim the result set down to the scanSize after performing the full
	// scan. This is an in-memory test double, so it's fast enough.
	getEmail := func(attrs dbAttributes) (email string, err error) {
		return (&dbParser{attrs}).GetString("email")
	}
	startScan := func(items []dbAttributes) ([]dbAttributes, error) {
		var lastEmail string

		if lastItem := input.ExclusiveStartKey; len(lastItem) == 0 {
			return items, nil
		} else if lastEmail, err = getEmail(lastItem); err != nil {
			return items, nil
		}
		for i, sub := range items {
			var email string
			if email, err = getEmail(sub); err != nil {
				return nil, err
			} else if email == lastEmail {
				return items[i+1:], nil
			}
		}
		return items, nil
	}
	endScan := func(
		items []dbAttributes, n int,
	) (result []dbAttributes, lastKey dbAttributes, err error) {
		if n == 0 || len(items) <= n {
			result = items
			return
		}
		items = items[:n]

		if lastEmail, err := getEmail(items[len(items)-1]); err == nil {
			result = items
			lastKey = dbAttributes{"email": &dbString{Value: lastEmail}}
		}
		return
	}

	n := client.scanSize
	var lastKey dbAttributes

	if items, err = startScan(items); err != nil {
		return
	} else if items, lastKey, err = endScan(items, n); err == nil {
		output = &dynamodb.ScanOutput{Items: items, LastEvaluatedKey: lastKey}
	}
	return
}

func newSubscriberRecord(sub *Subscriber) dbAttributes {
	return dbAttributes{
		"email":            &dbString{Value: sub.Email},
		"uid":              &dbString{Value: sub.Uid.String()},
		string(sub.Status): toDynamoDbTimestamp(sub.Timestamp),
	}
}

func TestDynamodDbMethodsReturnExternalErrorsAsAppropriate(t *testing.T) {
	client := &TestDynamoDbClient{}
	dyndb := &DynamoDb{client, "subscribers-table"}
	ctx := context.Background()

	checkIsExternalError := func(t *testing.T, err error) {
		t.Helper()
		assert.Check(t, testutils.ErrorIs(err, ops.ErrExternal))
	}

	// All these methods are tested in dynamodb_contract_test, and none of those
	// should result in external errors. So the TestDynamoDbClient
	// implementations are empty except to return simulated external errors
	// wrapped via ops.AwsError.
	//
	// The one exception is Scan(), which is tested more thoroughly below.
	client.SetServerError("simulated server error")

	err := dyndb.CreateTable(ctx)
	checkIsExternalError(t, err)

	_, err = dyndb.UpdateTimeToLive(ctx)
	checkIsExternalError(t, err)

	err = dyndb.DeleteTable(ctx)
	checkIsExternalError(t, err)

	_, err = dyndb.Get(ctx, testdata.TestEmail)
	checkIsExternalError(t, err)

	err = dyndb.Put(ctx, &Subscriber{})
	checkIsExternalError(t, err)

	err = dyndb.Delete(ctx, testdata.TestEmail)
	checkIsExternalError(t, err)
}

func TestGetAttribute(t *testing.T) {
	attrs := dbAttributes{
		"email":      &dbString{Value: testdata.TestEmail},
		"unexpected": &types.AttributeValueMemberBOOL{Value: false},
	}

	parseString := func(attr *dbString) (string, error) {
		return attr.Value, nil
	}

	t.Run("Succeeds", func(t *testing.T) {
		value, err := getAttribute("email", attrs, parseString)

		assert.NilError(t, err)
		assert.Equal(t, testdata.TestEmail, value)
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
			"email":    &dbString{Value: testdata.TestEmail},
			"uid":      &dbString{Value: testdata.TestUidStr},
			"verified": toDynamoDbTimestamp(testdata.TestTimestamp),
		}

		subscriber, err := parseSubscriber(attrs)

		assert.NilError(t, err)
		assert.DeepEqual(t, subscriber, &Subscriber{
			Email:     testdata.TestEmail,
			Uid:       testdata.TestUid,
			Status:    SubscriberVerified,
			Timestamp: testdata.TestTimestamp,
		})
	})

	t.Run("ErrorsIfGettingAttributesFail", func(t *testing.T) {
		subscriber, err := parseSubscriber(dbAttributes{})

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
			"uid":      &dbString{Value: testdata.TestUidStr},
			"pending":  toDynamoDbTimestamp(testdata.TestTimestamp),
			"verified": toDynamoDbTimestamp(testdata.TestTimestamp),
		}

		subscriber, err := parseSubscriber(attrs)

		assert.Check(t, is.Nil(subscriber))

		const errFmt = "contains both '%s' and '%s' attributes"
		expected := fmt.Sprintf(
			errFmt, SubscriberPending, SubscriberVerified,
		)
		assert.ErrorContains(t, err, expected)
	})

	t.Run("ErrorsIfTimestampIsNotAnInteger", func(t *testing.T) {
		attrs := dbAttributes{
			"email":    &dbString{Value: testdata.TestEmail},
			"uid":      &dbString{Value: testdata.TestUidStr},
			"verified": &dbNumber{Value: "not an int"},
		}

		subscriber, err := parseSubscriber(attrs)

		assert.Check(t, is.Nil(subscriber))
		assert.ErrorContains(t, err, "failed to parse 'verified' from: ")
	})
}

func setupDbWithSubscribers() (dyndb *DynamoDb, client *TestDynamoDbClient) {
	client = &TestDynamoDbClient{}
	dyndb = &DynamoDb{client, "subscribers-table"}

	client.addSubscribers(TestSubscribers)
	return
}

func TestProcessSubscribers(t *testing.T) {
	ctx := context.Background()

	setup := func() (
		dyndb *DynamoDb,
		client *TestDynamoDbClient,
		subs *[]*Subscriber,
		f SubscriberFunc,
	) {
		dyndb, client = setupDbWithSubscribers()
		subs = &[]*Subscriber{}
		f = SubscriberFunc(func(s *Subscriber) bool {
			*subs = append(*subs, s)
			return true
		})
		return
	}

	t.Run("Succeeds", func(t *testing.T) {
		t.Run("WithoutPagination", func(t *testing.T) {
			dynDb, client, subs, f := setup()

			err := dynDb.ProcessSubscribers(ctx, SubscriberVerified, f)

			assert.NilError(t, err)
			assert.DeepEqual(t, TestVerifiedSubscribers, *subs)
			assert.Equal(t, client.scanCalls, 1)
		})

		t.Run("WithPagination", func(t *testing.T) {
			dynDb, client, subs, f := setup()
			client.scanSize = 1

			err := dynDb.ProcessSubscribers(ctx, SubscriberVerified, f)

			assert.NilError(t, err)
			assert.DeepEqual(t, TestVerifiedSubscribers, *subs)
			assert.Equal(t, client.scanCalls, len(TestVerifiedSubscribers))
		})

		t.Run("WithoutProcessingAllSubscribers", func(t *testing.T) {
			dynDb, _, subs, _ := setup()
			f := SubscriberFunc(func(s *Subscriber) bool {
				*subs = append(*subs, s)
				return s.Email != TestVerifiedSubscribers[1].Email
			})

			err := dynDb.ProcessSubscribers(ctx, SubscriberVerified, f)

			assert.NilError(t, err)
			assert.DeepEqual(t, TestVerifiedSubscribers[:2], *subs)
		})
	})

	t.Run("ReturnsError", func(t *testing.T) {
		t.Run("IfScanFails", func(t *testing.T) {
			dynDb, client, _, f := setup()
			client.SetServerError("scanning error")

			err := dynDb.ProcessSubscribers(ctx, SubscriberVerified, f)

			assert.ErrorContains(t, err, "scanning error")
			assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
		})

		t.Run("IfParseSubscriberFails", func(t *testing.T) {
			dynDb, client, _, f := setup()
			status := SubscriberVerified
			client.addSubscriberRecord(dbAttributes{
				"email":        &dbString{Value: "bad-uid@foo.com"},
				"uid":          &dbString{Value: "not a uid"},
				string(status): toDynamoDbTimestamp(testdata.TestTimestamp),
			})

			err := dynDb.ProcessSubscribers(ctx, SubscriberVerified, f)

			expectedErr := "failed to parse subscriber: " +
				"failed to parse 'uid' from: "
			assert.ErrorContains(t, err, expectedErr)
			assert.Assert(t, testutils.ErrorIsNot(err, ops.ErrExternal))
		})
	})
}
