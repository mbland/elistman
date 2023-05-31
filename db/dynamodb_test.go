//go:build small_tests || all_tests

package db

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

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
// The original exception to this was Scan(), which was the reason why the
// DynamoDbClient interface was created. Testing all the cases of the code that
// relies on Scan() is annoying, difficult, and/or nearly impossible without
// using this test double.
//
// CreateTable, DescribeTable, and UpdateTimeToLive are also implemented. The
// dynamodb_contract_test tests and validates these individual operations. Given
// that, CreateSubscribersTable can then be tested more quickly and reliably
// using this test double.
type TestDynamoDbClient struct {
	serverErr         error
	createTableInput  *dynamodb.CreateTableInput
	createTableOutput *dynamodb.CreateTableOutput
	createTableErr    error
	descTableInput    *dynamodb.DescribeTableInput
	descTableOutput   *dynamodb.DescribeTableOutput
	descTableErr      error
	updateTtlInput    *dynamodb.UpdateTimeToLiveInput
	updateTtlOutput   *dynamodb.UpdateTimeToLiveOutput
	updateTtlErr      error
	subscribers       []dbAttributes
	scanSize          int
	scanCalls         int
	scanErr           error
}

func (client *TestDynamoDbClient) SetAllErrors(msg string) {
	err := testutils.AwsServerError(msg)
	client.serverErr = err
	client.createTableErr = err
	client.descTableErr = err
	client.updateTtlErr = err
	client.scanErr = err
}

func (client *TestDynamoDbClient) SetCreateTableError(msg string) {
	client.createTableErr = testutils.AwsServerError(msg)
}

func (client *TestDynamoDbClient) SetDescribeTableError(msg string) {
	client.descTableErr = testutils.AwsServerError(msg)
}

func (client *TestDynamoDbClient) SetUpdateTimeToLiveError(msg string) {
	client.updateTtlErr = testutils.AwsServerError(msg)
}

func (client *TestDynamoDbClient) SetScanError(msg string) {
	client.scanErr = testutils.AwsServerError(msg)
}

func (client *TestDynamoDbClient) CreateTable(
	_ context.Context,
	input *dynamodb.CreateTableInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.CreateTableOutput, error) {
	client.createTableInput = input
	return client.createTableOutput, client.createTableErr
}

func (client *TestDynamoDbClient) DescribeTable(
	_ context.Context,
	input *dynamodb.DescribeTableInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.DescribeTableOutput, error) {
	client.descTableInput = input
	return client.descTableOutput, client.descTableErr
}

func (client *TestDynamoDbClient) UpdateTimeToLive(
	_ context.Context,
	input *dynamodb.UpdateTimeToLiveInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.UpdateTimeToLiveOutput, error) {
	client.updateTtlInput = input
	return client.updateTtlOutput, client.updateTtlErr
}

func (client *TestDynamoDbClient) DeleteTable(
	context.Context, *dynamodb.DeleteTableInput, ...func(*dynamodb.Options),
) (*dynamodb.DeleteTableOutput, error) {
	return nil, client.serverErr
}

func (client *TestDynamoDbClient) GetItem(
	context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options),
) (*dynamodb.GetItemOutput, error) {
	return nil, client.serverErr
}

func (client *TestDynamoDbClient) PutItem(
	context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options),
) (*dynamodb.PutItemOutput, error) {
	return nil, client.serverErr
}

func (client *TestDynamoDbClient) DeleteItem(
	context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options),
) (*dynamodb.DeleteItemOutput, error) {
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

	err = client.scanErr
	if err != nil {
		return
	}

	// Remember that our schema is to keep pending and verified subscribers
	// partitioned across disjoint Global Secondary Indexes. So we first filter
	// for subscribers in the desired state.
	subscribers := make([]dbAttributes, 0, len(client.subscribers))
	for _, sub := range client.subscribers {
		if _, ok := sub[aws.ToString(input.IndexName)]; !ok {
			continue
		}
		subscribers = append(subscribers, sub)
	}

	// Scan starting just past the start key until we reach the scan limit.
	items := make([]dbAttributes, 0, len(subscribers))
	getEmail := func(attrs dbAttributes) (email string) {
		email, _ = (&dbParser{attrs}).GetString("email")
		return
	}
	startKey := getEmail(input.ExclusiveStartKey)
	started := len(startKey) == 0
	atScanLimit := func() bool {
		return client.scanSize != 0 && len(items) == client.scanSize
	}
	var lastKey dbAttributes

	for i, sub := range subscribers {
		if !started {
			started = getEmail(sub) == startKey
			continue
		}
		items = append(items, sub)

		if atScanLimit() {
			if i != (len(subscribers) - 1) {
				lastKey = dbAttributes{"email": sub["email"]}
			}
			break
		}
	}
	output = &dynamodb.ScanOutput{Items: items, LastEvaluatedKey: lastKey}
	return
}

func newSubscriberRecord(sub *Subscriber) dbAttributes {
	return dbAttributes{
		"email":            &dbString{Value: sub.Email},
		"uid":              &dbString{Value: sub.Uid.String()},
		string(sub.Status): toDynamoDbTimestamp(sub.Timestamp),
	}
}

func checkIsExternalError(t *testing.T, err error) {
	t.Helper()
	assert.Check(t, testutils.ErrorIs(err, ops.ErrExternal))
}

func TestDynamodDbMethodsReturnExternalErrorsAsAppropriate(t *testing.T) {
	client := &TestDynamoDbClient{}
	dyndb := &DynamoDb{client, "subscribers-table"}
	ctx := context.Background()

	// All these methods are tested in dynamodb_contract_test, and none of those
	// should result in external errors. So the TestDynamoDbClient
	// implementations are empty except to return simulated external errors
	// wrapped via ops.AwsError.
	//
	// The one exception is Scan(), which is tested more thoroughly below.
	client.SetAllErrors("simulated server error")

	err := dyndb.createTable(ctx)
	checkIsExternalError(t, err)

	_, err = dyndb.updateTimeToLive(ctx)
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

func TestCreateSubscribersTable(t *testing.T) {
	ctx := context.Background()
	setup := func() (dyndb *DynamoDb, client *TestDynamoDbClient) {
		client = &TestDynamoDbClient{
			descTableOutput: &dynamodb.DescribeTableOutput{
				Table: &types.TableDescription{
					TableStatus: types.TableStatusActive,
				},
			},
			updateTtlOutput: &dynamodb.UpdateTimeToLiveOutput{
				TimeToLiveSpecification: &types.TimeToLiveSpecification{},
			},
		}
		dyndb = &DynamoDb{Client: client, TableName: "subscribers"}
		return
	}

	assertAwsStringEqual := func(
		t *testing.T, expected string, actual *string,
	) {
		t.Helper()
		assert.Equal(t, expected, aws.ToString(actual))
	}

	t.Run("Succeeds", func(t *testing.T) {
		dyndb, client := setup()

		err := dyndb.CreateSubscribersTable(ctx, time.Nanosecond)

		assert.NilError(t, err)
		tableName := dyndb.TableName
		assertAwsStringEqual(t, tableName, client.createTableInput.TableName)
		assertAwsStringEqual(t, tableName, client.createTableInput.TableName)
		assertAwsStringEqual(t, tableName, client.updateTtlInput.TableName)
		ttlSpec := client.updateTtlInput.TimeToLiveSpecification
		assertAwsStringEqual(
			t, string(SubscriberPending), ttlSpec.AttributeName,
		)
		assert.Assert(t, aws.ToBool(ttlSpec.Enabled) == true)
	})

	t.Run("FailsIfCreateTableFails", func(t *testing.T) {
		dyndb, client := setup()
		client.SetCreateTableError("create table failed")

		err := dyndb.CreateSubscribersTable(ctx, time.Nanosecond)

		checkIsExternalError(t, err)
		assert.ErrorContains(t, err, "create table failed")
	})

	t.Run("FailsIfWaitForTableFails", func(t *testing.T) {
		dyndb, client := setup()
		client.SetDescribeTableError("describe table failed")

		err := dyndb.CreateSubscribersTable(ctx, time.Nanosecond)

		// Because WaitForTable uses dynamodb.TableExistsWaiter, it won't pass
		// through the DescribeTable error or its message. It will just fail.
		assert.ErrorContains(t, err, "failed waiting for subscribers table")
	})

	t.Run("FailsIfUpdateTimeToLiveFails", func(t *testing.T) {
		dyndb, client := setup()
		client.SetUpdateTimeToLiveError("update TTL failed")

		err := dyndb.CreateSubscribersTable(ctx, time.Nanosecond)

		checkIsExternalError(t, err)
		assert.ErrorContains(t, err, "update TTL failed")
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
			client.SetScanError("scanning error")

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
