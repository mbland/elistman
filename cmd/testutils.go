//go:build small_tests || all_tests

package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type CommandTestFixture struct {
	Cmd    *cobra.Command
	Stdout *strings.Builder
	Stderr *strings.Builder
}

func NewCommandTestFixture(cmd *cobra.Command) (f *CommandTestFixture) {
	f = &CommandTestFixture{
		Cmd: cmd, Stdout: &strings.Builder{}, Stderr: &strings.Builder{},
	}
	cmd.SetIn(bytes.NewReader([]byte{}))
	cmd.SetOut(f.Stdout)
	cmd.SetErr(f.Stderr)
	cmd.SetArgs([]string{})
	return
}

func (f *CommandTestFixture) ExecuteAndAssertStdoutContains(
	t *testing.T, expectedOutput string,
) {
	t.Helper()

	err := f.Cmd.Execute()

	assert.NilError(t, err)
	assert.Equal(t, "", f.Stderr.String())
	assert.Assert(t, is.Contains(f.Stdout.String(), expectedOutput))
}

func (f *CommandTestFixture) ExecuteAndAssertErrorContains(
	t *testing.T, expectedErrMsg string) {
	t.Helper()

	err := f.Cmd.Execute()

	assert.Equal(t, "", f.Stdout.String())
	assert.ErrorContains(t, err, expectedErrMsg)
	assert.Equal(t, fmt.Sprintf("Error: %s\n", err), f.Stderr.String())
}
