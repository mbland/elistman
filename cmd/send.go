// Copyright © 2023 Mike Bland <mbland@acm.org>
// See LICENSE.txt for details.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	ltypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/mbland/elistman/email"
	"github.com/spf13/cobra"
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send an email message to the mailing list",
	Long: `Reads a JSON object from standard input describing a message:

` + email.ExampleMessageJson + `

If the input passes validation, it then sends a copy of the message to each
mailing list member, customized with their unsubscribe URIs.

It takes one argument, the ARN of the Lambda function to invoke to send the
message.`,
	Args: cobra.ExactArgs(1),
	RunE: SendMessage,
}

func init() {
	rootCmd.AddCommand(sendCmd)
}

func SendMessage(cmd *cobra.Command, args []string) (err error) {
	lambdaArn := args[0]
	var cfg aws.Config
	var msg *email.Message

	if cfg, err = config.LoadDefaultConfig(context.Background()); err != nil {
		return
	} else if msg, err = email.NewMessageFromJson(os.Stdin); err != nil {
		return
	}

	evt := email.SendEvent{Message: *msg}
	client := lambda.NewFromConfig(cfg)
	var payload []byte

	if payload, err = json.Marshal(&evt); err != nil {
		err = fmt.Errorf("error creating Lambda payload: %s", err)
		return
	}

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
	if output, err = client.Invoke(context.Background(), input); err != nil {
		err = fmt.Errorf("error invoking Lambda function: %s", err)
	} else if output.StatusCode != http.StatusOK {
		const errFmt = "received non-200 response: %s"
		err = fmt.Errorf(errFmt, http.StatusText(int(output.StatusCode)))
	} else if output.FunctionError != nil {
		const errFmt = "error executing Lambda function: %s: %s"
		funcErr := aws.ToString(output.FunctionError)
		err = fmt.Errorf(errFmt, funcErr, string(output.Payload))
	} else if err = json.Unmarshal(output.Payload, &response); err != nil {
		const errFmt = "failed to unmarshal Lambda response payload: %s: %s"
		err = fmt.Errorf(errFmt, err, string(output.Payload))
	} else if !response.Success {
		const errFmt = "sending failed after sending to %d recipients: %s"
		err = fmt.Errorf(errFmt, response.NumSent, response.Details)
	} else {
		const successFmt = "Sent the message successfully to %d recipients.\n"
		fmt.Printf(successFmt, response.NumSent)
	}
	return
}
