//go:build small_tests || all_tests

package handler

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/events"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func setupTestCliHandler() (
	*cliHandler, *testAgent, *testutils.Logs, context.Context,
) {
	ta := &testAgent{}
	logs, logger := testutils.NewLogs()
	return &cliHandler{ta, logger}, ta, logs, context.Background()
}

func TestCliHandlerHandleSendEvent(t *testing.T) {
	event := &events.SendEvent{Message: *email.ExampleMessage}

	expectedLogMsg := func(
		msg *email.Message, res *events.SendResponse,
	) string {
		const logFmt = "send: subject: \"%s\"; success: %t; num sent: %d"
		return fmt.Sprintf(logFmt, msg.Subject, res.Success, res.NumSent)
	}

	t.Run("Succeeds", func(t *testing.T) {
		handler, agent, logs, ctx := setupTestCliHandler()
		agent.NumSent = 27

		res := handler.HandleSendEvent(ctx, event)

		expectedResult := &events.SendResponse{
			Success: true, NumSent: agent.NumSent,
		}
		assert.DeepEqual(t, expectedResult, res)
		logs.AssertContains(t, expectedLogMsg(&event.Message, expectedResult))
	})

	t.Run("FailsIfSendRaisesError", func(t *testing.T) {
		handler, agent, logs, ctx := setupTestCliHandler()
		agent.Error = errors.New("simulated Send error")

		res := handler.HandleSendEvent(ctx, event)

		expectedResult := &events.SendResponse{
			Success: false, Details: agent.Error.Error(),
		}
		assert.DeepEqual(t, expectedResult, res)
		logs.AssertContains(t, expectedLogMsg(&event.Message, expectedResult))
	})
}

func TestCliHandlerHandleEvent(t *testing.T) {
	t.Run("SuccessfullyHandlesSendEvent", func(t *testing.T) {
		handler, agent, _, ctx := setupTestCliHandler()
		event := &events.CommandLineEvent{
			EListManCommand: events.CommandLineSendEvent,
			Send:            &events.SendEvent{Message: *email.ExampleMessage},
		}
		agent.NumSent = 27

		res, err := handler.HandleEvent(ctx, event)

		assert.NilError(t, err)
		expectedResult := &events.SendResponse{
			Success: true, NumSent: agent.NumSent,
		}
		assert.DeepEqual(t, expectedResult, res)
	})

	t.Run("FailsOnUnknownEvent", func(t *testing.T) {
		handler, _, _, ctx := setupTestCliHandler()
		event := &events.CommandLineEvent{
			EListManCommand: events.CommandLineEventType("unknown"),
		}

		res, err := handler.HandleEvent(ctx, event)

		assert.Assert(t, is.Nil(res))
		assert.ErrorContains(t, err, "unknown EListMan command: unknown")
	})
}
