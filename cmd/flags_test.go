//go:build small_tests || all_tests

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/assert"
)

func TestGetStringFlag(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("test-flag", "", "flag for testing flags")

		err := cmd.ParseFlags([]string{"--test-flag", "foobar"})

		assert.NilError(t, err)
		assert.Equal(t, "foobar", getStringFlag(cmd, "test-flag"))
	})

	t.Run("ReturnsEmptyStringIfNotSpecified", func(t *testing.T) {
		assert.Equal(t, "", getStringFlag(&cobra.Command{}, "nonexistent-flag"))
	})
}

func TestStackNameFlag(t *testing.T) {
	cmd := &cobra.Command{}
	registerStackName(cmd)
	err := cmd.ParseFlags([]string{"-s", TestStackName})

	assert.NilError(t, err)
	assert.Equal(t, TestStackName, getStackName(cmd))
}
