//go:build small_tests || all_tests

package ops

import (
	"testing"

	"gotest.tools/assert"
)

func TestUnknownResult(t *testing.T) {
	unknownResult := Invalid - 1
	assert.Equal(t, "OperationResult(-1)", unknownResult.String())
}

func TestKnownResult(t *testing.T) {
	assert.Equal(t, "Subscribed", Subscribed.String())
}
