//go:build small_tests || all_tests

package cmd

import (
	"strings"
	"testing"

	"github.com/mbland/elistman/email"
	"gotest.tools/assert"
)

func TestPreview(t *testing.T) {
	setup := func() *CommandTestFixture {
		return NewCommandTestFixture(newPreviewCommand())
	}

	t.Run("SucceedsWithExample", func(t *testing.T) {
		f := setup()
		f.Cmd.SetArgs([]string{"--example"})

		f.ExecuteAndAssertStdoutContains(t, "Hello, World!")
		assert.Assert(t, f.Cmd.SilenceUsage == true)
	})

	t.Run("SucceedsWithStandardInput", func(t *testing.T) {
		f := setup()
		f.Cmd.SetIn(strings.NewReader(strings.ReplaceAll(
			email.ExampleMessageJson, "Hello, World!", "Hola, Mundo!",
		)))

		f.ExecuteAndAssertStdoutContains(t, "Hola, Mundo!")
	})

	t.Run("PassesThroughParseError", func(t *testing.T) {
		f := setup()
		f.Cmd.SetIn(strings.NewReader("not a JSON message object"))

		const expectedMsg = "failed to parse message input from JSON: "
		f.ExecuteAndAssertErrorContains(t, expectedMsg)
	})
}
