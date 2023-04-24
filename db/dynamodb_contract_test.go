//go:build medium_tests || contract_tests || all_tests

package db

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

var useAwsDb bool
var dynamodbDockerVersion string
var maxTableWaitAttempts int
var durationBetweenAttempts time.Duration

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
	flag.IntVar(
		&maxTableWaitAttempts,
		"dbwaitattempts",
		3,
		"Maximum times to wait for a new DynamoDB table to become active",
	)
	flag.DurationVar(
		&durationBetweenAttempts,
		"dbwaitattemptduration",
		5*time.Second,
		"Duration to wait between each DynamoDB table status check",
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
	maxAttempts := maxTableWaitAttempts
	sleep := func() { time.Sleep(durationBetweenAttempts) }
	doSetup := setupLocalDynamoDb
	ctx := context.Background()

	if useAwsDb == true {
		doSetup = setupAwsDynamoDb
	}

	if dynDb, teardownDb, err = doSetup(tableName); err != nil {
		return
	} else if err = dynDb.CreateTable(ctx); err != nil {
		err = teardownDbWithError(err)
	} else if err = dynDb.WaitForTable(ctx, maxAttempts, sleep); err != nil {
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
	config, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		err = fmt.Errorf("failed to configure DynamoDB: %s", err)
	} else {
		dynDb = NewDynamoDb(&config, tableName)
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
		dynDb = NewDynamoDb(config, tableName)
	}
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
	})

	t.Run("DeleteTableFailsIfTableDoesNotExist", func(t *testing.T) {
		err := badDb.DeleteTable(ctx)

		expected := "failed to delete db table " + badDb.TableName + ": "
		assert.ErrorContains(t, err, expected)
	})

	t.Run("PutGetAndDeleteSucceed", func(t *testing.T) {
		subscriber := newTestSubscriber()

		putErr := testDb.Put(ctx, subscriber)
		retrievedSubscriber, getErr := testDb.Get(ctx, subscriber.Email)
		deleteErr := testDb.Delete(ctx, subscriber.Email)
		_, getAfterDeleteErr := testDb.Get(ctx, subscriber.Email)

		assert.NilError(t, putErr)
		assert.NilError(t, getErr)
		assert.NilError(t, deleteErr)
		assert.DeepEqual(t, subscriber, retrievedSubscriber)
		expected := subscriber.Email + " is not a subscriber"
		assert.ErrorContains(t, getAfterDeleteErr, expected)
	})

	t.Run("DescribeTable", func(t *testing.T) {
		t.Run("Succeeds", func(t *testing.T) {
			td, err := testDb.DescribeTable(ctx)

			assert.NilError(t, err)
			assert.Equal(t, types.TableStatusActive, td.TableStatus)
		})

		t.Run("FailsIfTableDoesNotExist", func(t *testing.T) {
			td, err := badDb.DescribeTable(ctx)

			assert.Assert(t, is.Nil(td))
			errMsg := "failed to describe db table " + badDb.TableName
			assert.ErrorContains(t, err, errMsg)
			assert.ErrorContains(t, err, "ResourceNotFoundException")
		})
	})

	t.Run("WaitForTable", func(t *testing.T) {
		setup := func() (*int, func()) {
			numSleeps := 0
			return &numSleeps, func() { numSleeps++ }
		}

		t.Run("Succeeds", func(t *testing.T) {
			numSleeps, sleep := setup()

			err := testDb.WaitForTable(ctx, 1, sleep)

			assert.NilError(t, err)
			assert.Equal(t, 0, *numSleeps)
		})

		t.Run("ErrorsIfMaxAttemptsLessThanOne", func(t *testing.T) {
			numSleeps, sleep := setup()

			err := testDb.WaitForTable(ctx, 0, sleep)

			msg := "maxAttempts to wait for DB table must be >= 0, got: 0"
			assert.ErrorContains(t, err, msg)
			assert.Equal(t, 0, *numSleeps)
		})

		t.Run("ErrorsIfTableDoesNotBecomeActive", func(t *testing.T) {
			numSleeps, sleep := setup()
			maxAttempts := 3

			err := badDb.WaitForTable(ctx, maxAttempts, sleep)

			msg := fmt.Sprintf(
				"db table %s not active after %d attempts to check",
				badDb.TableName,
				maxAttempts,
			)
			assert.ErrorContains(t, err, msg)
			assert.ErrorContains(t, err, "ResourceNotFoundException")
			assert.Equal(t, maxAttempts-1, *numSleeps)
		})
	})

	t.Run("GetFails", func(t *testing.T) {
		t.Run("IfSubscriberDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			retrieved, err := testDb.Get(ctx, subscriber.Email)

			assert.Assert(t, is.Nil(retrieved))
			expected := subscriber.Email + " is not a subscriber"
			assert.ErrorContains(t, err, expected)
		})

		t.Run("IfTableDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			retrieved, err := badDb.Get(ctx, subscriber.Email)

			assert.Assert(t, is.Nil(retrieved))
			expected := "failed to get " + subscriber.Email + ": "
			assert.ErrorContains(t, err, expected)
		})
	})

	t.Run("PutFails", func(t *testing.T) {
		t.Run("IfTableDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			err := badDb.Put(ctx, subscriber)

			assert.ErrorContains(t, err, "failed to put "+subscriber.Email+": ")
		})
	})

	t.Run("DeleteFails", func(t *testing.T) {
		t.Run("IfTableDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			err := badDb.Delete(ctx, subscriber.Email)

			expected := "failed to delete " + subscriber.Email + ": "
			assert.ErrorContains(t, err, expected)
		})
	})
}
