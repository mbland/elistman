// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	ltypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
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
	rootCmd.AddCommand(newSendCmd(NewCloudFormationClient, NewLambdaClient))
}

func newSendCmd(
	newCloudFormationClient CloudFormationClientFactoryFunc,
	newLambdaClient LambdaClientFactoryFunc,
) *cobra.Command {
	return &cobra.Command{
		Use:   "send",
		Short: "Send an email message to the mailing list",
		Long:  sendDescription,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sendMessage(
				cmd, newCloudFormationClient(), newLambdaClient(), args[0],
			)
		},
	}
}

func mustMarshal(obj any, errPrefix string) (payload []byte) {
	var err error
	if payload, err = json.Marshal(obj); err != nil {
		panic(errPrefix + ": " + err.Error())
	}
	return
}

func sendMessage(
	cmd *cobra.Command,
	cfc CloudFormationClient,
	lc LambdaClient,
	stackName string,
) (err error) {
	cmd.SilenceUsage = true
	ctx := context.Background()
	var msg *email.Message
	var lambdaArn string

	if msg, err = email.NewMessageFromJson(cmd.InOrStdin()); err != nil {
		return
	} else if lambdaArn, err = GetLambdaArn(ctx, cfc, stackName); err != nil {
		return
	}

	evt := email.SendEvent{Message: *msg}
	payload := mustMarshal(&evt, "error creating Lambda payload")

	input := &lambda.InvokeInput{
		FunctionName: aws.String(lambdaArn),
		LogType:      ltypes.LogTypeTail,
		Payload:      payload,
	}
	var output *lambda.InvokeOutput
	var response email.SendResponse

	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/lambda#Client.Invoke
	// https://docs.aws.amazon.com/lambda/latest/dg/invocation-sync.html
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/lambda#InvokeInput
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/lambda#InvokeOutput
	if output, err = lc.Invoke(ctx, input); err != nil {
		return fmt.Errorf("error invoking Lambda function: %s", err)
	} else if output.StatusCode != http.StatusOK {
		const errFmt = "received non-200 response: %s"
		return fmt.Errorf(errFmt, http.StatusText(int(output.StatusCode)))
	} else if output.FunctionError != nil {
		const errFmt = "error executing Lambda function: %s: %s"
		funcErr := aws.ToString(output.FunctionError)
		return fmt.Errorf(errFmt, funcErr, string(output.Payload))
	} else if err = json.Unmarshal(output.Payload, &response); err != nil {
		const errFmt = "failed to unmarshal Lambda response payload: %s: %s"
		return fmt.Errorf(errFmt, err, string(output.Payload))
	} else if !response.Success {
		const errFmt = "sending failed after sending to %d recipients: %s"
		return fmt.Errorf(errFmt, response.NumSent, response.Details)
	} else {
		const successFmt = "Sent the message successfully to %d recipients.\n"
		cmd.Printf(successFmt, response.NumSent)
	}
	return
}
