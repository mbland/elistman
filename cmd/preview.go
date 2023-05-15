// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/mbland/elistman/email"
	"github.com/spf13/cobra"
)

var previewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview a raw email message without sending it",
	Long: `Reads a JSON object from standard input describing a message:

  {
    "from": "Foo Bar <foobar@example.com>",
    "subject": "Test object",
    "textBody": "Hello, World!",
    "textFooter": "Unsubscribe: {{UnsubscribeUrl}}",
    "htmlBody": "<!DOCTYPE html><html><head></head><body>Hello, World!<br/>",
    "htmlFooter": "<a href='{{UnsubscribeUrl}}'>Unsubscribe</a></body></html>"
  }

Emits a raw email message to standard output representing what would be sent to
each mailing list member.`,
	RunE: previewRawMessage,
}

func init() {
	rootCmd.AddCommand(previewCmd)
}

func previewRawMessage(cmd *cobra.Command, args []string) error {
	rawJson, err := io.ReadAll(os.Stdin)

	if err != nil {
		return fmt.Errorf("failed to read standard input: %w", err)
	}

	msg := &email.Message{}
	if err = json.Unmarshal(rawJson, msg); err != nil {
		return fmt.Errorf("failed to parse input as JSON message: %w", err)
	}

	msgTemplate := email.NewMessageTemplate(msg)
	sub := &email.Subscriber{
		Email: "subscribe@foo.com",
		Uid:   uuid.MustParse("00000000-1111-2222-3333-444444444444"),
	}
	sub.SetUnsubscribeInfo("unsubscribe@bar.com", "https://bar.com/email/")

	if err = msgTemplate.EmitMessage(os.Stdout, sub); err != nil {
		return fmt.Errorf("failed to emit preview message: %w", err)
	}
	return nil
}
