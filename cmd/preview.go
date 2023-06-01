// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"io"
	"strings"

	"github.com/mbland/elistman/email"
	"github.com/spf13/cobra"
)

const previewDescription = `` +
	`Reads a JSON object from standard input describing a message:

` + email.ExampleMessageJson + `

If the input passes validation, it then emits a raw email message to standard
output representing what would be sent to each mailing list member.`

func newPreviewCommand() *cobra.Command {
	var emitExample bool

	previewCmd := &cobra.Command{
		Use:   "preview",
		Short: "Preview a raw email message without sending it",
		Long:  previewDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var input io.Reader = cmd.InOrStdin()
			if emitExample {
				input = strings.NewReader(email.ExampleMessageJson)
			}
			return email.EmitPreviewMessageFromJson(input, cmd.OutOrStdout())
		},
	}
	previewCmd.Flags().BoolVarP(
		&emitExample, "example", "x", false,
		"Use the help example to generate the preview",
	)
	return previewCmd
}

func init() {
	rootCmd.AddCommand(newPreviewCommand())
}
