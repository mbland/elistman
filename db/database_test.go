package db

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"gotest.tools/assert"
)

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
