//go:build medium_tests || all_tests

package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"gotest.tools/assert"
)

var dbConfig *aws.Config

func TestMain(m *testing.M) {
	var teardown func() error
	var err error

	if dbConfig, teardown, err = setupDynamoDb(); err != nil {
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

func setupDynamoDb() (conf *aws.Config, teardown func() error, err error) {
	// TODO: Add logic to decide whether to test against local or remote
	// DynamoDB.
	//
	// Also create a single random table for all tests. Local teardown will stop
	// the Docker container. Remote teardown will drop the test table.
	return setupLocalDynamoDb()
}

// See also:
// - https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/DynamoDBLocal.DownloadingAndRunning.html
// - https://github.com/aws-samples/aws-sam-java-rest
// - https://hub.docker.com/r/amazon/dynamodb-local
// - https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/DynamoDBLocal.UsageNotes.html
func setupLocalDynamoDb() (conf *aws.Config, teardown func() error, err error) {
	var endpoint string
	var containerId string

	if err = checkDockerIsRunning(); err != nil {
		return
	} else if endpoint, err = pickUnusedEndpoint(); err != nil {
		return
	} else if containerId, err = launchLocalDb(endpoint); err != nil {
		return
	} else if conf, err = localDbConfig(endpoint); err != nil {
		return
	}
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
	if listener, err := net.Listen("tcp", ":0"); err != nil {
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

func newTestDatabase() *DynamoDb {
	cfg := aws.Config{}
	return NewDynamoDb(cfg, "TestTable")
}

// This function will be replaced by more substantial tests once I begin to
// implement DynamoDb.
func TestDatabaseInitialization(t *testing.T) {
	db := newTestDatabase()

	assert.Equal(t, db.TableName, "TestTable")
}
