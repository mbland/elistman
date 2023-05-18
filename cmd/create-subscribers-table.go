// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/mbland/elistman/db"
	"github.com/spf13/cobra"
)

var createSubscribersTableCmd = &cobra.Command{
	Use:   "create-subscribers-table",
	Short: "Create a DynamoDB table for mailing list subscribers",
	Long: `Creates a new DynamoDB table for mailing list subscriber information.

The new table will have an index for pending subscribers and an index for
verified subscribers. Pending subscribers will automatically expire after 24
hours, after which the DynamoDB Time To Live feature will remove them.

The command takes one argument, which is the name of the table to create. This
name will become the value of the SUBSCRIBERS_TABLE_NAME environment variable
used to configure and deploy the application.`,
	Args: cobra.ExactArgs(1),
	RunE: CreateSubscribersTable,
}

func init() {
	rootCmd.AddCommand(createSubscribersTableCmd)
}

func CreateSubscribersTable(cmd *cobra.Command, args []string) (err error) {
	tableName := args[0]
	ctx := context.Background()
	var cfg aws.Config

	if cfg, err = config.LoadDefaultConfig(ctx); err != nil {
		return
	}

	dbase := &db.DynamoDb{
		Client: dynamodb.NewFromConfig(cfg), TableName: tableName,
	}

	if err = dbase.CreateTable(ctx); err != nil {
		return
	} else if err = dbase.WaitForTable(ctx, time.Minute); err != nil {
		return
	} else if _, err = dbase.UpdateTimeToLive(ctx); err != nil {
		return
	}
	fmt.Printf("Successfully created DynamoDB table: %s\n", tableName)
	return
}
