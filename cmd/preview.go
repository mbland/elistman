// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"io"
	"os"
	"strings"

	"github.com/mbland/elistman/email"
	"github.com/spf13/cobra"
)

var emitExample bool

var previewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview a raw email message without sending it",
	Long: `Reads a JSON object from standard input describing a message:

` + email.ExampleMessageJson + `

If the input passes validation, it then emits a raw email message to standard
output representing what would be sent to each mailing list member.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var input io.Reader = os.Stdin
		if emitExample {
			input = strings.NewReader(email.ExampleMessageJson)
		}
		return email.EmitPreviewMessageFromJson(input, os.Stdout)
	},
}

func init() {
	previewCmd.Flags().BoolVarP(
		&emitExample, "example", "x", false,
		"Use the help example to generate the preview",
	)
	rootCmd.AddCommand(previewCmd)
}
