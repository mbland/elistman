//go:build small_tests || all_tests

package cmd

import (
	"fmt"
	"testing"

	"github.com/mbland/elistman/db"
	"gotest.tools/assert"
)

func TestCreateSubscribersTable(t *testing.T) {
	const TableName = "elistman-subscribers"

	setup := func() (f *CommandTestFixture, client *db.TestDynamoDbClient) {
		client = db.NewTestDynamoDbClient()
		f = NewCommandTestFixture(
			newCreateSubscribersTableCmd(func(tableName string) *db.DynamoDb {
				return &db.DynamoDb{Client: client, TableName: tableName}
			}),
		)
		f.Cmd.SetArgs([]string{"elistman-subscribers"})
		return
	}

	t.Run("Succeeds", func(t *testing.T) {
		f, _ := setup()

		const outFmt = "Successfully created DynamoDB table: %s\n"
		f.ExecuteAndAssertStdoutContains(t, fmt.Sprintf(outFmt, TableName))
		assert.Assert(t, f.Cmd.SilenceUsage == true)
	})

	t.Run("FailsOnDynamodDbClientError", func(t *testing.T) {
		f, client := setup()
		client.SetCreateTableError("create table test error")

		f.ExecuteAndAssertErrorContains(t, "create table test error")
	})
}
