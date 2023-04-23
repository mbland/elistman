//go:build medium_tests || contract_tests || all_tests

package db

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

var useAwsDb bool
var maxTableWaitAttempts int
var durationBetweenAttempts time.Duration

func init() {
	flag.BoolVar(
		&useAwsDb,
		"awsdb",
		false,
		"Test against DynamoDB in AWS (instead of local Docker container)",
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

	tableName := "elistman-database-test-" + randomString(10)
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

// Inspired by:
// - https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
func randomString(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"0123456789_"

	result := make([]byte, n)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
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

const dbImage = "amazon/dynamodb-local"

// See also:
// - https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/DynamoDBLocal.DownloadingAndRunning.html
// - https://github.com/aws-samples/aws-sam-java-rest
// - https://hub.docker.com/r/amazon/dynamodb-local
// - https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/DynamoDBLocal.UsageNotes.html
func setupLocalDynamoDb(
	tableName string,
) (dynDb *DynamoDb, teardown func() error, err error) {
	var endpoint string
	var containerId string
	var config *aws.Config

	if err = checkDockerIsRunning(); err != nil {
		return
	} else if endpoint, err = pickUnusedEndpoint(); err != nil {
		return
	} else if err = pullDynamoDbDockerImage(); err != nil {
		return
	} else if containerId, err = launchLocalDb(endpoint); err != nil {
		return
	} else if config, err = localDbConfig(endpoint); err != nil {
		return
	}
	dynDb = NewDynamoDb(config, tableName)
	teardown = func() error { return cleanupLocalDb(containerId) }
	return
}

func checkDockerIsRunning() (err error) {
	cmd := exec.Command("docker", "info")
	if err = cmd.Run(); err != nil {
		err = errors.New("Please start Docker before running this test.")
	}
	return
}

func pullDynamoDbDockerImage() error {
	cmd := exec.Command("docker", "pull", dbImage)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull %s: %s:\n%s", dbImage, err, output)
	}
	return nil
}

func pickUnusedEndpoint() (string, error) {
	if listener, err := net.Listen("tcp", "localhost:0"); err != nil {
		return "", errors.New("failed to pick unused endpoint: " + err.Error())
	} else {
		listener.Close()
		return listener.Addr().String(), nil
	}
}

func launchLocalDb(localEndpoint string) (string, error) {
	portMap := localEndpoint + ":8000"
	cmd := exec.Command("docker", "run", "-d", "-p", portMap, dbImage)

	if output, err := cmd.CombinedOutput(); err != nil {
		const errFmt = "failed to start local DynamoDB at %s: %s:\n%s"
		return "", fmt.Errorf(errFmt, localEndpoint, err, output)
	} else {
		const logFmt = "local DynamoDB running at %s with container ID: %s"
		containerId := string(output)
		log.Printf(logFmt, localEndpoint, containerId)
		return strings.TrimSpace(containerId), nil
	}
}

func cleanupLocalDb(containerId string) (errResult error) {
	stopCmd := exec.Command("docker", "stop", "-t", "0", containerId)
	rmCmd := exec.Command("docker", "rm", containerId)

	raiseErr := func(action string, err error, output []byte) error {
		const errFmt = "failed to %s local DynamoDB container %s: %s:\n%s"
		return fmt.Errorf(errFmt, action, containerId, err, output)
	}

	log.Printf("stopping local DynamoDB with container ID: " + containerId)

	if output, err := stopCmd.CombinedOutput(); err != nil {
		errResult = raiseErr("stop", err, output)
	} else if output, err = rmCmd.CombinedOutput(); err != nil {
		errResult = raiseErr("remove", err, output)
	}
	return
}

// Inspired by:
// - https://davidagood.com/dynamodb-local-go/
// - https://github.com/aws/aws-sdk-go-v2/blob/main/config/example_test.go
func localDbConfig(localEndpoint string) (*aws.Config, error) {
	localResolver := aws.EndpointResolverWithOptionsFunc(
		func(
			service, region string, options ...interface{},
		) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "http://" + localEndpoint}, nil
		},
	)
	dbConfig, err := config.LoadDefaultConfig(
		context.Background(),
		// From: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     "AKID",
				SecretAccessKey: "SECRET",
				SessionToken:    "SESSION",
				Source:          "example hard coded credentials",
			},
		}),
		config.WithRegion("local"),
		config.WithEndpointResolverWithOptions(localResolver),
	)
	if err != nil {
		const errFmt = "failed to configure local DynamoDB at: %s: %s"
		return nil, fmt.Errorf(errFmt, localEndpoint, err)
	}
	return &dbConfig, nil
}

func newTestSubscriber() *Subscriber {
	return NewSubscriber(randomString(8) + "@example.com")
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
