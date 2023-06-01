//go:build small_tests || all_tests

package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mbland/elistman/db"
	"github.com/spf13/cobra"
	"gotest.tools/assert"
)

func TestCreateSubscribersTable(t *testing.T) {
	const TableName = "elistman-subscribers"

	setup := func() (
		cmd *cobra.Command,
		stdout *strings.Builder,
		stderr *strings.Builder,
		client *db.TestDynamoDbClient,
	) {
		client = db.NewTestDynamoDbClient()
		cmd, stdout, stderr = SetupCommandForTesting(
			newCreateSubscribersTableCmd(func(tableName string) *db.DynamoDb {
				return &db.DynamoDb{Client: client, TableName: tableName}
			}),
		)
		cmd.SetArgs([]string{"elistman-subscribers"})
		return
	}

	t.Run("Succeeds", func(t *testing.T) {
		cmd, stdout, stderr, _ := setup()

		err := cmd.Execute()

		assert.NilError(t, err)
		assert.Assert(t, cmd.SilenceUsage == true)
		const outFmt = "Successfully created DynamoDB table: %s\n"
		assert.Equal(t, fmt.Sprintf(outFmt, TableName), stdout.String())
		assert.Equal(t, "", stderr.String())
	})

	t.Run("FailsOnDynamodDbClientError", func(t *testing.T) {
		cmd, stdout, stderr, client := setup()
		client.SetCreateTableError("create table test error")

		err := cmd.Execute()

		assert.ErrorContains(t, err, "create table test error")
		assert.Equal(t, "", stdout.String())
		assert.Equal(t, fmt.Sprintf("Error: %s\n", err), stderr.String())
	})
}
