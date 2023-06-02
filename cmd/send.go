// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"context"
	"fmt"

	"github.com/mbland/elistman/email"
	"github.com/spf13/cobra"
)

const sendDescription = `` +
	`Reads a JSON object from standard input describing a message:

` + email.ExampleMessageJson + `

If the input passes validation, it then sends a copy of the message to each
mailing list member, customized with their unsubscribe URIs.

It takes one argument, the STACK_NAME of the EListMan instance.`

func init() {
	rootCmd.AddCommand(newSendCmd(NewEListManFunc))
}

func newSendCmd(newFunc EListManFactoryFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "send",
		Short: "Send an email message to the mailing list",
		Long:  sendDescription,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sendMessage(cmd, newFunc(args[0]))
		},
	}
}

func sendMessage(cmd *cobra.Command, elistmanFunc EListManFunc) (err error) {
	cmd.SilenceUsage = true
	ctx := context.Background()
	var msg *email.Message

	if msg, err = email.NewMessageFromJson(cmd.InOrStdin()); err != nil {
		return
	}

	evt := &email.SendEvent{Message: *msg}
	var response email.SendResponse

	if err = elistmanFunc.Invoke(ctx, evt, &response); err != nil {
		return fmt.Errorf("sending failed: %w", err)
	} else if !response.Success {
		const errFmt = "sending failed after sending to %d recipients: %s"
		return fmt.Errorf(errFmt, response.NumSent, response.Details)
	} else {
		const successFmt = "Sent the message successfully to %d recipients.\n"
		cmd.Printf(successFmt, response.NumSent)
	}
	return
}
