//go:build small_tests || all_tests

package cmd

import (
	"strings"
	"testing"

	"gotest.tools/assert"
)

func TestCmdExecute(t *testing.T) {
	origRootCmd := *rootCmd
	defer func() {
		*rootCmd = origRootCmd
	}()

	output := &strings.Builder{}
	rootCmd.SetArgs([]string{})
	rootCmd.SetOut(output)

	err := Execute()

	assert.NilError(t, err)
	assert.Assert(t, len(output.String()) != 0)
}
