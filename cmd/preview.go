// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"os"

	"github.com/mbland/elistman/email"
	"github.com/spf13/cobra"
)

var previewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview a raw email message without sending it",
	Long: `Reads a JSON object from standard input describing a message:

` + email.ExampleMessageJson + `

If the input passes validation, it then emits a raw email message to standard
output representing what would be sent to each mailing list member.`,
	RunE: previewRawMessage,
}

func init() {
	rootCmd.AddCommand(previewCmd)
}

func previewRawMessage(cmd *cobra.Command, args []string) (err error) {
	return email.EmitPreviewMessageFromJson(os.Stdin, os.Stdout)
}
