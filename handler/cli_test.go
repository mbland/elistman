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
		agent.NumSent = 27

		res, err := handler.HandleEvent(ctx, event)

		assert.NilError(t, err)
		expectedResult := &events.SendResponse{
			Success: true, NumSent: agent.NumSent,
		}
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
