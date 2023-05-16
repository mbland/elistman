//go:build small_tests || all_tests

package types

import (
	"testing"

	"gotest.tools/assert"
)

const errTesting = SentinelError("testing")

func TestSentinelError(t *testing.T) {
	t.Run("ReturnsItselfFromErrorMethod", func(t *testing.T) {
		assert.Equal(t, string(errTesting), errTesting.Error())
	})
}
