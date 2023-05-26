//go:build small_tests || all_tests

package types

import (
	"testing"

	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestCapacity(t *testing.T) {
	t.Run("CreatedSuccessfully", func(t *testing.T) {
		cap := NewCapacity(0.5)

		assert.Equal(t, 0.5, cap.Value())
		assert.Equal(t, "50.00%", cap.String())
		assert.Equal(t, 50, cap.MaxAvailable(100))
	})

	t.Run("CreatedSuccessfullyAtUpperAndLowerBounds", func(t *testing.T) {
		lowerCap := NewCapacity(0.0)
		upperCap := NewCapacity(1.0)

		assert.Equal(t, 0.0, lowerCap.cap)
		assert.Equal(t, 1.0, upperCap.cap)
	})

	t.Run("PanicsIfNegative", func(t *testing.T) {
		defer testutils.ExpectPanic(t, "got: -0.1")

		cap := NewCapacity(-0.1)

		assert.Assert(t, is.Nil(cap))
	})

	t.Run("PanicsIfGreaterThanOne", func(t *testing.T) {
		defer testutils.ExpectPanic(t, "got: 1.1")

		cap := NewCapacity(1.1)

		assert.Assert(t, is.Nil(cap))
	})
}
