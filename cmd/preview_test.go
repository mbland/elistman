//go:build small_tests || all_tests

package cmd

import (
	"strings"
	"testing"

	"github.com/mbland/elistman/email"
	"github.com/spf13/cobra"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestPreview(t *testing.T) {
	setup := func() (cmd *cobra.Command, stdout, stderr *strings.Builder) {
		return SetupCommandForTesting(newPreviewCommand())
	}

	t.Run("SucceedsWithExample", func(t *testing.T) {
		cmd, stdout, stderr := setup()
		cmd.SetArgs([]string{"--example"})

		err := cmd.Execute()

		assert.NilError(t, err)
		assert.Assert(t, cmd.SilenceUsage == true)
		assert.Equal(t, "", stderr.String())
		assert.Assert(t, is.Contains(stdout.String(), "Hello, World!"))
	})

	t.Run("SucceedsWithStandardInput", func(t *testing.T) {
		cmd, stdout, stderr := setup()
		cmd.SetIn(strings.NewReader(strings.ReplaceAll(
			email.ExampleMessageJson, "Hello, World!", "Hola, Mundo!",
		)))

		err := cmd.Execute()

		assert.NilError(t, err)
		assert.Equal(t, "", stderr.String())
		assert.Assert(t, is.Contains(stdout.String(), "Hola, Mundo!"))
	})

	t.Run("PassesThroughParseError", func(t *testing.T) {
		cmd, stdout, stderr := setup()
		cmd.SetIn(strings.NewReader("not a JSON message object"))

		err := cmd.Execute()

		assert.Equal(t, "", stdout.String())
		const expectedMsg = "failed to parse message input from JSON: "
		assert.ErrorContains(t, err, expectedMsg)
		assert.Assert(t, is.Contains(stderr.String(), expectedMsg))
	})
}
