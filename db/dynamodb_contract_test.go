//go:build ((medium_tests || contract_tests) && !no_coverage_tests) || coverage_tests || all_tests

package db

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

var useAwsDb bool
var dynamodbDockerVersion string
var maxTableWaitDuration time.Duration

func init() {
	flag.BoolVar(
		&useAwsDb,
		"awsdb",
		false,
		"Test against DynamoDB in AWS (instead of local Docker container)",
	)
	flag.StringVar(
		&dynamodbDockerVersion,
		"dynDbDockerVersion",
		"1.21.0",
		"Version of the amazon/dynamodb-local Docker image to test against",
	)
	flag.DurationVar(
		&maxTableWaitDuration,
		"dbwaitduration",
		1*time.Minute,
		"Maximum duration to wait for DynamoDB table to become active",
	)
}

func setupDynamoDb() (dynDb *DynamoDb, teardown func() error, err error) {
	var teardownDb func() error
	teardownDbWithError := func(err error) error {
		if err == nil {
			return teardownDb()
		} else if teardownErr := teardownDb(); teardownErr != nil {
			const msgFmt = "teardown after error failed: %s\noriginal error: %s"
			return fmt.Errorf(msgFmt, teardownErr, err)
		}
		return err
	}

	tableName := "elistman-database-test-" + testutils.RandomString(10)
	doSetup := setupLocalDynamoDb
	ctx := context.Background()

	if useAwsDb == true {
		doSetup = setupAwsDynamoDb
	}

	if dynDb, teardownDb, err = doSetup(tableName); err != nil {
		return
	} else if err = dynDb.CreateTable(ctx); err != nil {
		err = teardownDbWithError(err)
	} else if err = dynDb.WaitForTable(ctx, maxTableWaitDuration); err != nil {
		err = teardownDbWithError(err)
	} else {
		teardown = func() error {
			return teardownDbWithError(dynDb.DeleteTable(ctx))
		}
	}
	return
}

func setupAwsDynamoDb(
	tableName string,
) (dynDb *DynamoDb, teardown func() error, err error) {
	var cfg aws.Config

	if cfg, err = ops.LoadDefaultAwsConfig(); err == nil {
		dynDb = &DynamoDb{dynamodb.NewFromConfig(cfg), tableName}
		teardown = func() error { return nil }
	}
	return
}

// See also:
// - https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/DynamoDBLocal.DownloadingAndRunning.html
// - https://github.com/aws-samples/aws-sam-java-rest
// - https://hub.docker.com/r/amazon/dynamodb-local
// - https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/DynamoDBLocal.UsageNotes.html
func setupLocalDynamoDb(
	tableName string,
) (dynDb *DynamoDb, teardown func() error, err error) {
	config, endpoint, err := localDbConfig()
	if err != nil {
		return
	}

	dockerImage := "amazon/dynamodb-local:" + dynamodbDockerVersion
	teardown, err = testutils.LaunchDockerContainer(
		dynamodb.ServiceID, endpoint, 8000, dockerImage,
	)
	if err == nil {
		dynDb = &DynamoDb{dynamodb.NewFromConfig(*config), tableName}
	}

	// Wait a second for the container to become ready. This avoids errors like
	// the following, which have occurred both locally and in the CI/CD
	// pipeline:
	//
	//   operation error DynamoDB: CreateTable, exceeded maximum number of
	//   attempts, 3
	time.Sleep(1 * time.Second)
	return
}

// Inspired by:
// - https://davidagood.com/dynamodb-local-go/
// - https://github.com/aws/aws-sdk-go-v2/blob/main/config/example_test.go
// - https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/endpoints/
func localDbConfig() (*aws.Config, string, error) {
	dbConfig, resolver, err := testutils.AwsConfig()
	if err != nil {
		const errFmt = "failed to configure local DynamoDB: %s"
		return nil, "", fmt.Errorf(errFmt, err)
	}

	endpoint, err := resolver.CreateEndpoint(dynamodb.ServiceID)
	if err != nil {
		return nil, "", err
	}
	return dbConfig, endpoint, nil
}

func newTestSubscriber() *Subscriber {
	return NewSubscriber(testutils.RandomString(8) + "@example.com")
}

func sorted(subs []*Subscriber) (r []*Subscriber) {
	r = make([]*Subscriber, len(subs))
	copy(r, subs)
	sort.Slice(r, func(i, j int) bool {
		return r[i].Email < r[j].Email
	})
	return
}

func TestDynamoDb(t *testing.T) {
	testDb, teardown, err := setupDynamoDb()

	assert.NilError(t, err)
	defer func() {
		err := teardown()
		assert.NilError(t, err)
	}()

	ctx := context.Background()
	var badDb DynamoDb = *testDb
	badDb.TableName = testDb.TableName + "-nonexistent"

	// Note that the success cases for CreateTable and DeleteTable are
	// confirmed by setupDynamoDb() and teardown() above.
	t.Run("CreateTableFailsIfTableExists", func(t *testing.T) {
		err := testDb.CreateTable(ctx)

		expected := "failed to create db table " + testDb.TableName + ": "
		assert.ErrorContains(t, err, expected)
		assert.Assert(t, testutils.ErrorIsNot(err, ops.ErrExternal))
	})

	t.Run("DeleteTableFailsIfTableDoesNotExist", func(t *testing.T) {
		err := badDb.DeleteTable(ctx)

		expected := "failed to delete db table " + badDb.TableName + ": "
		assert.ErrorContains(t, err, expected)
		assert.Assert(t, testutils.ErrorIsNot(err, ops.ErrExternal))
	})

	t.Run("PutGetAndDeleteSucceed", func(t *testing.T) {
		subscriber := newTestSubscriber()

		putErr := testDb.Put(ctx, subscriber)
		retrievedSubscriber, getErr := testDb.Get(ctx, subscriber.Email)
		deleteErr := testDb.Delete(ctx, subscriber.Email)
		_, getAfterDeleteErr := testDb.Get(ctx, subscriber.Email)
		deleteAfterDeleteErr := testDb.Delete(ctx, subscriber.Email)

		assert.NilError(t, putErr)
		assert.NilError(t, getErr)
		assert.NilError(t, deleteErr)
		assert.DeepEqual(t, subscriber, retrievedSubscriber)
		assert.Assert(
			t, testutils.ErrorIs(getAfterDeleteErr, ErrSubscriberNotFound),
		)
		// Believe it or not, deleting a nonexistent record doesn't raise any
		// kind of an error.
		assert.NilError(t, deleteAfterDeleteErr)
	})

	t.Run("UpdateTimeToLive", func(t *testing.T) {
		t.Run("Succeeds", func(t *testing.T) {
			ttlSpec, err := testDb.UpdateTimeToLive(ctx)

			assert.NilError(t, err)
			expectedAttrName := string(SubscriberPending)
			actualAttrName := aws.ToString(ttlSpec.AttributeName)
			assert.Equal(t, expectedAttrName, actualAttrName)
			assert.Equal(t, true, aws.ToBool(ttlSpec.Enabled))
		})

		t.Run("FailsIfTableDoesNotExist", func(t *testing.T) {
			ttlSpec, err := badDb.UpdateTimeToLive(ctx)

			assert.Assert(t, is.Nil(ttlSpec))
			expectedErr := "failed to update Time To Live: " +
				"operation error DynamoDB: UpdateTimeToLive"
			assert.ErrorContains(t, err, expectedErr)
			assert.Assert(t, testutils.ErrorIsNot(err, ops.ErrExternal))
		})
	})

	t.Run("GetFails", func(t *testing.T) {
		t.Run("IfSubscriberDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			retrieved, err := testDb.Get(ctx, subscriber.Email)

			assert.Assert(t, is.Nil(retrieved))
			assert.Assert(t, testutils.ErrorIs(err, ErrSubscriberNotFound))
			assert.Assert(t, testutils.ErrorIsNot(err, ops.ErrExternal))
		})

		t.Run("IfTableDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			retrieved, err := badDb.Get(ctx, subscriber.Email)

			assert.Assert(t, is.Nil(retrieved))
			expected := "failed to get " + subscriber.Email + ": "
			assert.ErrorContains(t, err, expected)
			assert.Assert(t, testutils.ErrorIsNot(err, ops.ErrExternal))
		})
	})

	t.Run("PutFails", func(t *testing.T) {
		t.Run("IfTableDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			err := badDb.Put(ctx, subscriber)

			assert.ErrorContains(t, err, "failed to put "+subscriber.Email+": ")
			assert.Assert(t, testutils.ErrorIsNot(err, ops.ErrExternal))
		})
	})

	t.Run("DeleteFails", func(t *testing.T) {
		t.Run("IfTableDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			err := badDb.Delete(ctx, subscriber.Email)

			expected := "failed to delete " + subscriber.Email + ": "
			assert.ErrorContains(t, err, expected)
			assert.Assert(t, testutils.ErrorIsNot(err, ops.ErrExternal))
		})
	})

	t.Run("CountSubscribersFails", func(t *testing.T) {
		t.Run("IfTableDoesNotExist", func(t *testing.T) {
			const status = SubscriberVerified

			count, err := badDb.CountSubscribers(ctx, status)

			assert.Equal(t, int64(-1), count)
			expectedErrPrefix := "failed to count " + string(status) +
				" subscribers: "
			assert.ErrorContains(t, err, expectedErrPrefix)
			assert.Assert(t, testutils.ErrorIsNot(err, ops.ErrExternal))
		})
	})

	t.Run("WithTestSubscribers", func(t *testing.T) {
		emails := make([]string, 0, len(TestSubscribers))

		for _, sub := range TestSubscribers {
			if err := testDb.Put(ctx, sub); err != nil {
				t.Fatalf("failed to put subscriber: %s", sub)
			}
			emails = append(emails, sub.Email)
		}

		if useAwsDb {
			time.Sleep(time.Duration(3 * time.Second))
		}

		defer func() {
			for _, email := range emails {
				if err := testDb.Delete(ctx, email); err != nil {
					t.Fatalf("failed to delete subscriber: %s", email)
				}
			}
		}()

		t.Run("CountSubscribersSucceeds", func(t *testing.T) {
			count, err := testDb.CountSubscribers(ctx, SubscriberVerified)

			assert.NilError(t, err)
			assert.Equal(t, int64(len(TestVerifiedSubscribers)), count)
		})

		t.Run("ProcessSubscribersInStateSucceeds", func(t *testing.T) {
			subs := &[]*Subscriber{}
			f := SubscriberFunc(func(s *Subscriber) bool {
				*subs = append(*subs, s)
				return true
			})

			err := testDb.ProcessSubscribersInState(ctx, SubscriberVerified, f)

			assert.NilError(t, err)
			assert.DeepEqual(t, sorted(TestVerifiedSubscribers), sorted(*subs))
		})
	})
}
