//go:build small_tests || all_tests

package ops

import (
	"testing"

	"gotest.tools/assert"
)

func TestSentinelError(t *testing.T) {
	t.Run("ReturnsItselfFromErrorMethod", func(t *testing.T) {
		assert.Equal(t, string(ErrExternal), ErrExternal.Error())
	})
}
