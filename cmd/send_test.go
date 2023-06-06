//go:build small_tests || all_tests

package cmd

import (
	"strings"
	"testing"

	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/events"
	"gotest.tools/assert"
)

func TestSend(t *testing.T) {
	setup := func() (f *CommandTestFixture, lambda *TestEListManFunc) {
		lambda = NewTestEListManFunc()
		f = NewCommandTestFixture(newSendCmd(lambda.GetFactoryFunc()))
		f.Cmd.SetIn(strings.NewReader(email.ExampleMessageJson))
		f.Cmd.SetArgs([]string{"-s", TestStackName})
		return
	}

	t.Run("Succeeds", func(t *testing.T) {
		f, lambda := setup()
		lambda.SetResponseJson(`{"Success": true, "NumSent": 27}`)

		const expectedOut = "Sent the message successfully to 27 recipients.\n"
		f.ExecuteAndAssertStdoutContains(t, expectedOut)

		assert.Assert(t, f.Cmd.SilenceUsage == true)
		expectedReq := &events.CommandLineEvent{
			EListManCommand: events.CommandLineSendEvent,
			Send:            &events.SendEvent{Message: *email.ExampleMessage},
		}
		lambda.AssertMatches(t, TestStackName, expectedReq)
	})

	t.Run("RequiresStackNameFlag", func(t *testing.T) {
		f, _ := setup()
		f.AssertFailsIfRequiredFlagMissing(t, FlagStackName, []string{})
	})

	t.Run("FailsIfCannotParseInput", func(t *testing.T) {
		f, _ := setup()
		f.Cmd.SetIn(strings.NewReader("not a message input"))

		const expectedErr = "failed to parse message input from JSON: "
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})

	t.Run("FailsIfInvokingLambdaFails", func(t *testing.T) {
		f, lambda := setup()
		f.AssertReturnsLambdaError(t, lambda, "sending failed: ")
	})

	t.Run("FailsIfSendingFailed", func(t *testing.T) {
		f, lambda := setup()
		lambda.SetResponseJson(
			`{"Success": false, "NumSent": 9, "Details": "test failure"}`,
		)

		const expectedErr = "sending failed after sending to 9 recipients: " +
			"test failure"
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})
}
