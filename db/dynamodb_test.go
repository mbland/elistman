//go:build small_tests || all_tests

package db

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testdata"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

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
		client = NewTestDynamoDbClient()
		dyndb = &DynamoDb{Client: client, TableName: "subscribers"}
		return
	}

	assertAwsStringEqual := func(
		t *testing.T, expected string, actual *string,
	) {
		t.Helper()
		assert.Equal(t, expected, aws.ToString(actual))
	}

	const createTableErrPrefix = "failed to create " +
		"subscribers table \"subscribers\": "

	assertExternalErrorContains := func(
		t *testing.T, err error, opPrefix, msg string,
	) {
		t.Helper()

		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))

		if len(opPrefix) != 0 {
			opPrefix += ": "
		}
		const errFmt = createTableErrPrefix + "%s%s: api error : %s"
		expected := fmt.Sprintf(errFmt, opPrefix, ops.ErrExternal, msg)
		assert.ErrorContains(t, err, expected)
	}

	assertErrorContains := func(
		t *testing.T, err error, opPrefix, msg string,
	) {
		t.Helper()

		if len(opPrefix) != 0 {
			opPrefix += ": "
		}
		expected := fmt.Sprintf(createTableErrPrefix+"%s%s", opPrefix, msg)
		assert.ErrorContains(t, err, expected)
	}

	t.Run("Succeeds", func(t *testing.T) {
		dyndb, client := setup()

		err := dyndb.CreateSubscribersTable(ctx, time.Nanosecond)

		assert.NilError(t, err)
		tableName := dyndb.TableName
		assertAwsStringEqual(t, tableName, client.CreateTableInput.TableName)
		assertAwsStringEqual(t, tableName, client.CreateTableInput.TableName)
		assertAwsStringEqual(t, tableName, client.UpdateTtlInput.TableName)
		ttlSpec := client.UpdateTtlInput.TimeToLiveSpecification
		assertAwsStringEqual(
			t, string(SubscriberPending), ttlSpec.AttributeName,
		)
		assert.Assert(t, aws.ToBool(ttlSpec.Enabled) == true)
	})

	t.Run("FailsIfCreateTableFails", func(t *testing.T) {
		dyndb, client := setup()
		client.SetCreateTableError("create table failed")

		err := dyndb.CreateSubscribersTable(ctx, time.Nanosecond)

		assertExternalErrorContains(t, err, "", "create table failed")
	})

	t.Run("FailsIfWaitForTableFails", func(t *testing.T) {
		dyndb, client := setup()
		client.SetDescribeTableError("describe table failed")

		err := dyndb.CreateSubscribersTable(ctx, time.Nanosecond)

		// Because WaitForTable uses dynamodb.TableExistsWaiter, it won't pass
		// through the DescribeTable error or its message. It will just fail.
		const errFmt = "failed waiting for table to become active after %s"
		assertErrorContains(t, err, fmt.Sprintf(errFmt, time.Nanosecond), "")
	})

	t.Run("FailsIfUpdateTimeToLiveFails", func(t *testing.T) {
		dyndb, client := setup()
		client.SetUpdateTimeToLiveError("update TTL failed")

		err := dyndb.CreateSubscribersTable(ctx, time.Nanosecond)

		const opPrefix = "failed to update Time To Live"
		assertExternalErrorContains(t, err, opPrefix, "update TTL failed")
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
			assert.Equal(t, client.ScanCalls, 1)
		})

		t.Run("WithPagination", func(t *testing.T) {
			dynDb, client, subs, f := setup()
			client.ScanSize = 1

			err := dynDb.ProcessSubscribers(ctx, SubscriberVerified, f)

			assert.NilError(t, err)
			assert.DeepEqual(t, TestVerifiedSubscribers, *subs)
			assert.Equal(t, client.ScanCalls, len(TestVerifiedSubscribers))
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
