// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"context"
	"fmt"

	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/events"
	"github.com/spf13/cobra"
)

const sendDescription = `` +
	`Reads a JSON object from standard input describing a message:

` + email.ExampleMessageJson + `

If the input passes validation, it then sends a copy of the message to each
mailing list member, customized with their unsubscribe URIs.

It takes one argument, the STACK_NAME of the EListMan instance.`

func init() {
	rootCmd.AddCommand(newSendCmd(NewEListManLambda))
}

func newSendCmd(newFunc EListManFactoryFunc) (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "send",
		Short: "Send an email message to the mailing list",
		Long:  sendDescription,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			return sendMessage(cmd, newFunc, getStackName(cmd))
		},
	}
	registerStackName(cmd)
	cmd.MarkFlagRequired(FlagStackName)
	return
}

func sendMessage(
	cmd *cobra.Command, newFunc EListManFactoryFunc, stackName string,
) (err error) {
	cmd.SilenceUsage = true
	var msg *email.Message
	var elistmanFunc EListManFunc

	if msg, err = email.NewMessageFromJson(cmd.InOrStdin()); err != nil {
		return
	} else if elistmanFunc, err = newFunc(stackName); err != nil {
		return
	}

	ctx := context.Background()
	evt := &events.CommandLineEvent{
		EListManCommand: events.CommandLineSendEvent,
		Send:            &events.SendEvent{Message: *msg},
	}
	var response events.SendResponse

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
