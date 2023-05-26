package testutils

import (
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func ExpectPanic(t *testing.T, expectedMsg string) {
	t.Helper()

	if r := recover(); r != nil {
		assert.Assert(t, is.Contains(r, expectedMsg))
	} else {
		t.Fatal("expected panic, but didn't")
	}
}
