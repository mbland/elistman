// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"context"
	"time"

	"github.com/mbland/elistman/db"
	"github.com/spf13/cobra"
)

const createSubscribersTableDescription = `` +
	`Creates a new DynamoDB table for mailing list subscriber information.

The new table will have an index for pending subscribers and an index for
verified subscribers. Pending subscribers will automatically expire after 24
hours, after which the DynamoDB Time To Live feature will remove them.

The command takes one argument, which is the name of the table to create. This
name will become the value of the SUBSCRIBERS_TABLE_NAME environment variable
used to configure and deploy the application.`

func init() {
	rootCmd.AddCommand(newCreateSubscribersTableCmd(NewDynamoDb))
}

func newCreateSubscribersTableCmd(newDynDb DynamoDbFactoryFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "create-subscribers-table",
		Short: "Create a DynamoDB table for mailing list subscribers",
		Long:  createSubscribersTableDescription,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return createSubscribersTable(cmd, newDynDb(args[0]), time.Minute)
		},
	}
}

func createSubscribersTable(
	cmd *cobra.Command, dyndb *db.DynamoDb, maxWaitDuration time.Duration,
) (err error) {
	cmd.SilenceUsage = true
	ctx := context.Background()

	if err = dyndb.CreateSubscribersTable(ctx, maxWaitDuration); err == nil {
		cmd.Printf("Successfully created DynamoDB table: %s\n", dyndb.TableName)
	}
	return
}
