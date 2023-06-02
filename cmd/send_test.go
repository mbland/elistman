//go:build small_tests || all_tests

package cmd

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	tu "github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestSend(t *testing.T) {
	setup := func() (
		f *CommandTestFixture,
		cfc *TestCloudFormationClient,
		tlc *TestLambdaClient,
	) {
		cfc = NewTestCloudFormationClient()

		tlc = NewTestLambdaClient()
		tlc.InvokeOutput.StatusCode = http.StatusOK
		tlc.InvokeOutput.Payload = []byte(`{"Success": true, "NumSent": 27}`)

		f = NewCommandTestFixture(
			newSendCmd(
				func() CloudFormationClient { return cfc },
				func() LambdaClient { return tlc },
			),
		)
		f.Cmd.SetIn(strings.NewReader(email.ExampleMessageJson))
		f.Cmd.SetArgs([]string{TestStackName})
		return
	}

	t.Run("Succeeds", func(t *testing.T) {
		f, _, tlc := setup()

		const expectedOut = "Sent the message successfully to 27 recipients.\n"
		f.ExecuteAndAssertStdoutContains(t, expectedOut)

		assert.Assert(t, f.Cmd.SilenceUsage == true)
		invokeFunctionName := tlc.InvokeInput.FunctionName
		tu.AssertAwsStringEqual(t, TestFunctionArn, invokeFunctionName)
		payload := bytes.NewReader(tlc.InvokeInput.Payload)
		actualMsg := email.MustParseMessageFromJson(payload)
		assert.DeepEqual(t, email.ExampleMessage, actualMsg)
	})

	t.Run("FailsIfCannotParseInput", func(t *testing.T) {
		f, _, _ := setup()
		f.Cmd.SetIn(strings.NewReader("not a message input"))

		const expectedErr = "failed to parse message input from JSON: "
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})

	t.Run("FailsIfCreatingNewLambdaFails", func(t *testing.T) {
		f, cfc, _ := setup()
		cfc.DescribeStacksOutput.Stacks = []cftypes.Stack{}

		f.ExecuteAndAssertErrorContains(t, "stack not found: "+TestStackName)
	})

	t.Run("FailsIfInvokingLambdaFails", func(t *testing.T) {
		f, _, tlc := setup()
		tlc.InvokeError = testutils.AwsServerError("invoke failed")

		err := f.ExecuteAndAssertErrorContains(t, "sending failed: ")

		assert.ErrorContains(t, err, "invoke failed")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})

	t.Run("FailsIfSendingFailed", func(t *testing.T) {
		f, _, tlc := setup()
		tlc.InvokeOutput.Payload = []byte(
			`{"Success": false, "NumSent": 9, "Details": "test failure"}`,
		)

		const expectedErr = "sending failed after sending to 9 recipients: " +
			"test failure"
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})
}
