// Copyright © 2023 Mike Bland <mbland@acm.org>.
// See LICENSE.txt for details.

package cmd

import (
	"github.com/spf13/cobra"
)

const elistmanDesc = "Mailing list system providing address validation " +
	"and unsubscribe URIs"
const elistmanDescLong = elistmanDesc + "\n\n" +
	`See the https://github.com/mbland/elistman README for details.

To create a table:
  elistman create-subscribers-table <TABLE_NAME>

To see an example of the message input JSON structure:
  elistman preview --help

To preview a raw message before sending, where ` + "`generate-email`" + ` is any
program that creates message input JSON:
  generate-email | elistman preview

To send an email to the list, given the ARN of the elistman Lambda function:
  generate-email | elistman send <LAMBDA_ARN>
`

var rootCmd = &cobra.Command{
	Use:     "elistman",
	Version: "v0.1.0",
	Short:   elistmanDesc,
	Long:    elistmanDescLong,
}

func Execute() error {
	return rootCmd.Execute()
}
