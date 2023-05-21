// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/ops"
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
	rootCmd.AddCommand(
		newCreateSubscribersTableCmd(ops.LoadDefaultAwsConfig, NewDynamoDb),
	)
}

func newCreateSubscribersTableCmd(
	loadAwsConfig AwsConfigFactoryFunc, newDynamodDb DynamoDbFactoryFunc,
) *cobra.Command {
	runFunc := func(cfg aws.Config, cmd *cobra.Command, args []string) error {
		tableName := args[0]
		cmd.SilenceUsage = true
		dyndb := newDynamodDb(cfg, tableName)
		return createSubscribersTable(cmd, dyndb, time.Minute)
	}

	return &cobra.Command{
		Use:   "create-subscribers-table",
		Short: "Create a DynamoDB table for mailing list subscribers",
		Long:  createSubscribersTableDescription,
		Args:  cobra.ExactArgs(1),
		RunE:  NewAwsCommandFunc(loadAwsConfig, runFunc),
	}
}

func createSubscribersTable(
	cmd *cobra.Command, dyndb *db.DynamoDb, maxWaitDuration time.Duration,
) (err error) {
	ctx := context.Background()

	if err = dyndb.CreateTable(ctx); err != nil {
		const errFmt = "failed to create subscribers table \"%s\": %w"
		err = fmt.Errorf(errFmt, dyndb.TableName, err)
	} else if err = dyndb.WaitForTable(ctx, maxWaitDuration); err != nil {
		const errFmt = "failed waiting for subscribers table \"%s\" for %s: %w"
		err = fmt.Errorf(errFmt, dyndb.TableName, maxWaitDuration, err)
	} else if _, err = dyndb.UpdateTimeToLive(ctx); err != nil {
		const errFmt = "failed updating Time To Live " +
			"for subscribers table \"%s\": %w"
		err = fmt.Errorf(errFmt, dyndb.TableName, err)
	} else {
		cmd.Printf("Successfully created DynamoDB table: %s\n", dyndb.TableName)
	}
	return
}
