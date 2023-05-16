// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"fmt"
	"os"

	"github.com/google/uuid"
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
	mt, err := email.NewListMessageTemplateFromJson(os.Stdin)

	if err != nil {
		return err
	}

	sub := &email.Recipient{
		Email: "subscriber@foo.com",
		Uid:   uuid.MustParse("00000000-1111-2222-3333-444444444444"),
	}
	sub.SetUnsubscribeInfo("unsubscribe@bar.com", "https://bar.com/email/")

	if err = mt.EmitMessage(os.Stdout, sub); err != nil {
		return fmt.Errorf("failed to emit preview message: %w", err)
	}
	return nil
}
