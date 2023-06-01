//go:build small_tests || all_tests

package cmd

import (
	"strings"

	"github.com/spf13/cobra"
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
