//go:build medium_tests || contract_tests || all_tests

package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

var testDb *DynamoDb

func TestMain(m *testing.M) {
	var teardown func() error
	var err error

	if testDb, teardown, err = setupDynamoDb(); err != nil {
		log.Print(err.Error())
		os.Exit(1)
	}

	retval := m.Run()
	if err := teardown(); err != nil {
		log.Print(err.Error())
		retval = 1
	}
	os.Exit(retval)
}

func setupDynamoDb() (dynDb *DynamoDb, teardown func() error, err error) {
	tableName := "elistman-database-test-" + randomString(10)
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

	// TODO(mbland): Add logic to decide whether to test against local or remote
	// DynamoDB.
	if dynDb, teardownDb, err = setupLocalDynamoDb(tableName); err != nil {
		return
	} else if err = dynDb.CreateTable(); err != nil {
		err = teardownDbWithError(err)
		return
	}

	teardown = func() error {
		return teardownDbWithError(dynDb.DeleteTable())
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
	cmd := exec.Command(
		"docker", "run", "-d", "-p", portMap, "amazon/dynamodb-local",
	)

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
		context.TODO(), config.WithEndpointResolverWithOptions(localResolver),
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

func TestDatabase(t *testing.T) {
	assert.Assert(t, testDb != nil)

	var badDb DynamoDb = *testDb
	badDb.TableName = testDb.TableName + "-nonexistent"

	// Note that the success cases for CreateTable and DeleteTable are confirmed
	// by TestMain().
	t.Run("CreateTableFailsIfTableExists", func(t *testing.T) {
		err := testDb.CreateTable()

		expected := "failed to create db table " + testDb.TableName + ": "
		assert.ErrorContains(t, err, expected)
	})

	t.Run("DeleteTableFailsIfTableDoesNotExist", func(t *testing.T) {
		err := badDb.DeleteTable()

		expected := "failed to delete db table " + badDb.TableName + ": "
		assert.ErrorContains(t, err, expected)
	})

	t.Run("PutGetAndDeleteSucceed", func(t *testing.T) {
		subscriber := newTestSubscriber()

		putErr := testDb.Put(subscriber)
		retrievedSubscriber, getErr := testDb.Get(subscriber.Email)
		deleteErr := testDb.Delete(subscriber.Email)
		_, getAfterDeleteErr := testDb.Get(subscriber.Email)

		assert.NilError(t, putErr)
		assert.NilError(t, getErr)
		assert.NilError(t, deleteErr)
		assert.DeepEqual(t, subscriber, retrievedSubscriber)
		expected := subscriber.Email + " is not a subscriber"
		assert.ErrorContains(t, getAfterDeleteErr, expected)
	})

	t.Run("GetFails", func(t *testing.T) {
		t.Run("IfSubscriberDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			retrieved, err := testDb.Get(subscriber.Email)

			assert.Assert(t, is.Nil(retrieved))
			expected := subscriber.Email + " is not a subscriber"
			assert.ErrorContains(t, err, expected)
		})

		t.Run("IfTableDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			retrieved, err := badDb.Get(subscriber.Email)

			assert.Assert(t, is.Nil(retrieved))
			expected := "failed to get " + subscriber.Email + ": "
			assert.ErrorContains(t, err, expected)
		})
	})

	t.Run("PutFails", func(t *testing.T) {
		t.Run("IfTableDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			err := badDb.Put(subscriber)

			assert.ErrorContains(t, err, "failed to put "+subscriber.Email+": ")
		})
	})

	t.Run("DeleteFails", func(t *testing.T) {
		t.Run("IfTableDoesNotExist", func(t *testing.T) {
			subscriber := newTestSubscriber()

			err := badDb.Delete(subscriber.Email)

			expected := "failed to delete " + subscriber.Email + ": "
			assert.ErrorContains(t, err, expected)
		})
	})
}
