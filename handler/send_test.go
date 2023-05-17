//go:build small_tests || all_tests

package handler

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mbland/elistman/email"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestSendHandlerHandleEvent(t *testing.T) {
	setup := func() (
		*sendHandler, *testAgent, *testutils.Logs, context.Context,
	) {
		ta := &testAgent{}
		logs, logger := testutils.NewLogs()
		return &sendHandler{ta, logger}, ta, logs, context.Background()
	}

	event := &email.SendEvent{Message: *email.ExampleMessage}

	expectedLogMsg := func(msg *email.Message, res *email.SendResponse) string {
		const logFmt = "send: subject: \"%s\"; success: %t; num sent: %d"
		return fmt.Sprintf(logFmt, msg.Subject, res.Success, res.NumSent)
	}

	t.Run("Succeeds", func(t *testing.T) {
		handler, agent, logs, ctx := setup()
		agent.NumSent = 27

		res := handler.HandleEvent(ctx, event)

		expectedResult := &email.SendResponse{
			Success: true, NumSent: agent.NumSent,
		}
		assert.DeepEqual(t, expectedResult, res)
		logs.AssertContains(t, expectedLogMsg(&event.Message, expectedResult))
	})

	t.Run("FailsIfSendRaisesError", func(t *testing.T) {
		handler, agent, logs, ctx := setup()
		agent.Error = errors.New("simulated Send error")

		res := handler.HandleEvent(ctx, event)

		expectedResult := &email.SendResponse{
			Success: false, Details: agent.Error.Error(),
		}
		assert.DeepEqual(t, expectedResult, res)
		logs.AssertContains(t, expectedLogMsg(&event.Message, expectedResult))
	})
}
