//go:build small_tests || all_tests

package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/assert"
)

func SetupCommandForTesting(
	command *cobra.Command,
) (cmd *cobra.Command, stdout, stderr *strings.Builder) {
	cmd = command
	stdout = &strings.Builder{}
	stderr = &strings.Builder{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{})
	return
}

func AssertExecuteError(
	t *testing.T,
	cmd *cobra.Command,
	stdout, stderr *strings.Builder,
	expectedOutput string,
) {
	t.Helper()

	err := cmd.Execute()

	assert.Equal(t, "", stdout.String())
	assert.ErrorContains(t, err, expectedOutput)
	assert.Equal(t, fmt.Sprintf("Error: %s\n", err), stderr.String())
}
