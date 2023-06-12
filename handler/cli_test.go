//go:build small_tests || all_tests

package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	ta := &testAgent{
		ImportedAddresses: make([]string, 0, 10),
		ImportResponse:    func(string) error { return nil },
	}
	logs, logger := testutils.NewLogs()
	return &cliHandler{ta, logger}, ta, logs, context.Background()
}

func TestCliHandlerHandleSendEvent(t *testing.T) {
	event := &events.SendEvent{Message: *email.ExampleMessage}
	targetedEvent := *event
	targetedEvent.Addresses = []string{"test@foo.com", "test@bar.com"}

	expectedLogMsg := func(
		msg *email.Message, res *events.SendResponse,
	) string {
		const logFmt = "send: subject: \"%s\"; success: %t; num sent: %d"
		return fmt.Sprintf(logFmt, msg.Subject, res.Success, res.NumSent)
	}

	t.Run("SucceedsSendingToList", func(t *testing.T) {
		handler, agent, logs, ctx := setupTestCliHandler()
		numSent := 27
		agent.SendResponse = func(_ *email.Message) (int, error) {
			return numSent, nil
		}

		res := handler.HandleSendEvent(ctx, event)

		expectedResult := &events.SendResponse{Success: true, NumSent: numSent}
		assert.DeepEqual(t, expectedResult, res)
		logs.AssertContains(t, expectedLogMsg(&event.Message, expectedResult))
		expectedCalls := []testAgentCalls{{Method: "Send", Msg: &event.Message}}
		assert.DeepEqual(t, expectedCalls, agent.Calls)
	})

	t.Run("SucceedsSendingToSpecificAddresses", func(t *testing.T) {
		handler, agent, logs, ctx := setupTestCliHandler()
		agent.SendTargetedResponse = func(
			_ *email.Message, addrs []string,
		) (int, error) {
			return len(targetedEvent.Addresses), nil
		}

		res := handler.HandleSendEvent(ctx, &targetedEvent)

		expectedResult := &events.SendResponse{
			Success: true, NumSent: len(targetedEvent.Addresses),
		}
		assert.DeepEqual(t, expectedResult, res)
		logs.AssertContains(t, expectedLogMsg(&event.Message, expectedResult))
		expectedCalls := []testAgentCalls{
			{
				Method: "SendTargeted",
				Msg:    &event.Message,
				Addrs:  targetedEvent.Addresses,
			},
		}
		assert.DeepEqual(t, expectedCalls, agent.Calls)
	})

	t.Run("FailsIfSendRaisesError", func(t *testing.T) {
		handler, agent, logs, ctx := setupTestCliHandler()
		sendErr := errors.New("simulated Send error")
		agent.SendResponse = func(_ *email.Message) (int, error) {
			// Pretend one of the sends succeeded, to make sure NumSent is set
			// properly.
			return 1, sendErr
		}

		res := handler.HandleSendEvent(ctx, event)

		expectedResult := &events.SendResponse{
			Success: false, Details: sendErr.Error(), NumSent: 1,
		}
		assert.DeepEqual(t, expectedResult, res)
		logs.AssertContains(t, expectedLogMsg(&event.Message, expectedResult))
	})

	t.Run("FailsIfSendTargetedRaisesError", func(t *testing.T) {
		handler, agent, logs, ctx := setupTestCliHandler()
		sendTargetedErr := errors.New("simulated SendTargeted error")
		agent.SendTargetedResponse = func(
			_ *email.Message, addrs []string,
		) (int, error) {
			// Pretend one of the sends succeeded, to make sure NumSent is set
			// properly.
			return 1, sendTargetedErr
		}

		res := handler.HandleSendEvent(ctx, &targetedEvent)

		expectedResult := &events.SendResponse{
			Success: false, Details: sendTargetedErr.Error(), NumSent: 1,
		}
		assert.DeepEqual(t, expectedResult, res)
		logs.AssertContains(t, expectedLogMsg(&event.Message, expectedResult))
	})
}

func TestCliHandlerHandleImportEvent(t *testing.T) {
	event := &events.ImportEvent{
		Addresses: []string{"foo@test.com", "bar@test.com", "baz@test.com"},
	}

	t.Run("Succeeds", func(t *testing.T) {
		handler, _, logs, ctx := setupTestCliHandler()

		res := handler.HandleImportEvent(ctx, event)

		expectedResponse := &events.ImportResponse{
			NumImported: len(event.Addresses),
		}
		assert.DeepEqual(t, expectedResponse, res)
		logs.AssertContains(t, fmt.Sprintf(
			"imported %d: %s",
			len(event.Addresses),
			strings.Join(event.Addresses, ", ")))
	})

	t.Run("EmptyEventDoesNothing", func(t *testing.T) {
		handler, _, logs, ctx := setupTestCliHandler()
		emptyEvent := &events.ImportEvent{}

		res := handler.HandleImportEvent(ctx, emptyEvent)

		assert.DeepEqual(t, &events.ImportResponse{}, res)
		assert.Equal(t, "", logs.Builder.String())
	})

	t.Run("ReportsFailures", func(t *testing.T) {
		handler, agent, logs, ctx := setupTestCliHandler()
		agent.ImportResponse = func(address string) (err error) {
			if address != "bar@test.com" {
				err = errors.New("test error")
			}
			return
		}

		res := handler.HandleImportEvent(ctx, event)

		expectedResponse := &events.ImportResponse{
			NumImported: 1,
			Failures: []string{
				"foo@test.com: test error",
				"baz@test.com: test error",
			},
		}
		assert.DeepEqual(t, expectedResponse, res)
		logs.AssertContains(t, "imported 1: bar@test.com")
		logs.AssertContains(t, fmt.Sprintf(
			"failed to import %d:\n  %s",
			len(expectedResponse.Failures),
			strings.Join(expectedResponse.Failures, "\n  "),
		))
	})
}

func TestCliHandlerHandleEvent(t *testing.T) {
	t.Run("SuccessfullyHandlesSendEvent", func(t *testing.T) {
		handler, agent, _, ctx := setupTestCliHandler()
		event := &events.CommandLineEvent{
			EListManCommand: events.CommandLineSendEvent,
			Send:            &events.SendEvent{Message: *email.ExampleMessage},
		}
		numSent := 27
		agent.SendResponse = func(_ *email.Message) (int, error) {
			return numSent, nil
		}

		res, err := handler.HandleEvent(ctx, event)

		assert.NilError(t, err)
		expectedResult := &events.SendResponse{Success: true, NumSent: numSent}
		assert.DeepEqual(t, expectedResult, res)
	})

	t.Run("SuccessfullyHandlesImportEvent", func(t *testing.T) {
		handler, _, _, ctx := setupTestCliHandler()
		event := &events.CommandLineEvent{
			EListManCommand: events.CommandLineImportEvent,
			Import: &events.ImportEvent{
				Addresses: []string{
					"foo@test.com", "bar@test.com", "baz@test.com",
				},
			},
		}

		res, err := handler.HandleEvent(ctx, event)

		assert.NilError(t, err)
		expectedResponse := &events.ImportResponse{
			NumImported: len(event.Import.Addresses),
		}
		assert.DeepEqual(t, expectedResponse, res)
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
