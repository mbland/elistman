//go:build small_tests || all_tests

package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestSend(t *testing.T) {
	setup := func() (f *CommandTestFixture, lambda *TestEListManFunc) {
		lambda = &TestEListManFunc{InvokeResJson: []byte{}}
		f = NewCommandTestFixture(
			newSendCmd(func(stackName string) (EListManFunc, error) {
				lambda.StackName = stackName
				return lambda, lambda.CreateFuncError
			}),
		)
		f.Cmd.SetIn(strings.NewReader(email.ExampleMessageJson))
		f.Cmd.SetArgs([]string{TestStackName})
		return
	}

	t.Run("Succeeds", func(t *testing.T) {
		f, lambda := setup()
		lambda.InvokeResJson = []byte(`{"Success": true, "NumSent": 27}`)

		const expectedOut = "Sent the message successfully to 27 recipients.\n"
		f.ExecuteAndAssertStdoutContains(t, expectedOut)

		assert.Assert(t, f.Cmd.SilenceUsage == true)
		assert.Equal(t, TestStackName, lambda.StackName)
		req, isSendEvent := lambda.InvokeReq.(*email.SendEvent)
		assert.Assert(t, isSendEvent == true)
		expectedReq := &email.SendEvent{Message: *email.ExampleMessage}
		assert.DeepEqual(t, expectedReq, req)
	})

	t.Run("FailsIfCannotParseInput", func(t *testing.T) {
		f, _ := setup()
		f.Cmd.SetIn(strings.NewReader("not a message input"))

		const expectedErr = "failed to parse message input from JSON: "
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})

	t.Run("FailsIfCreatingLambdaFails", func(t *testing.T) {
		f, lambda := setup()
		const errFmt = "%w: creating lambda failed"
		lambda.CreateFuncError = fmt.Errorf(errFmt, ops.ErrExternal)

		err := f.ExecuteAndAssertErrorContains(t, "creating lambda failed")

		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})

	t.Run("FailsIfInvokingLambdaFails", func(t *testing.T) {
		f, lambda := setup()
		lambda.InvokeError = fmt.Errorf("%w: invoke failed", ops.ErrExternal)

		err := f.ExecuteAndAssertErrorContains(t, "sending failed: ")

		assert.ErrorContains(t, err, "invoke failed")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})

	t.Run("FailsIfSendingFailed", func(t *testing.T) {
		f, lambda := setup()
		lambda.InvokeResJson = []byte(
			`{"Success": false, "NumSent": 9, "Details": "test failure"}`,
		)

		const expectedErr = "sending failed after sending to 9 recipients: " +
			"test failure"
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})
}
