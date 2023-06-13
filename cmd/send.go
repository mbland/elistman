// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/mail"

	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/events"
	"github.com/spf13/cobra"
)

const sendDescription = `` +
	`Reads a JSON object from standard input describing a message:

` + email.ExampleMessageJson + `

It first validates the message input and reports any errors. It then sends a
copy of the message to verified mailing list subscribers, customized with their
unsubscribe URIs.

If no subscriber addresses are specified on the command line, it sends the
message to all currently verified subscribers.

If subscriber addresses are specified, it will attempt to parse each one, and if
any fail, it will report the errors without sending the message. If all
addresses parse successfully, it will attempt to send the message to only those
addresses. The EListMan Lambda will perform further validation, and will only
send the message to addresses matching verified subscribers. It will send the
message to every verified subscriber address and report errors for all other
addresses.`

func init() {
	rootCmd.AddCommand(newSendCmd(NewEListManLambda))
}

func newSendCmd(newFunc EListManFactoryFunc) (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "send [address...]",
		Short: "Send an email message to the mailing list",
		Long:  sendDescription,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, argv []string) (err error) {
			return sendMessage(cmd, newFunc, getStackName(cmd), argv)
		},
	}
	registerStackName(cmd)
	cmd.MarkFlagRequired(FlagStackName)
	return
}

func sendMessage(
	cmd *cobra.Command,
	newFunc EListManFactoryFunc,
	stackName string,
	addrs []string,
) (err error) {
	cmd.SilenceUsage = true
	var msg *email.Message

	if msg, err = email.NewMessageFromJson(cmd.InOrStdin()); err != nil {
		return
	}

	if len(addrs) == 0 {
		addrs = nil
	} else if err = checkAddresses(addrs); err != nil {
		return
	}

	ctx := context.Background()
	evt := &events.CommandLineEvent{
		EListManCommand: events.CommandLineSendEvent,
		Send:            &events.SendEvent{Addresses: addrs, Message: *msg},
	}
	response := &events.SendResponse{}

	if err = newFunc.Invoke(ctx, stackName, evt, response); err != nil {
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

func checkAddresses(addrs []string) (err error) {
	errs := make([]error, 0, len(addrs))

	for _, addr := range addrs {
		if _, err = mail.ParseAddress(addr); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", addr, err))
		}
	}

	if err = errors.Join(errs...); err != nil {
		err = fmt.Errorf("recipient list includes invalid addresses:\n%w", err)
	}
	return
}
