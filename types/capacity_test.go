//go:build small_tests || all_tests

package types

import (
	"testing"

	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestNewCapacity(t *testing.T) {
	t.Run("CreatedSuccessfully", func(t *testing.T) {
		cap, err := NewCapacity(0.5)

		assert.NilError(t, err)
		assert.Equal(t, 0.5, cap.Value())
		assert.Equal(t, "50.00%", cap.String())
		assert.Equal(t, 50, cap.MaxAvailable(100))
	})

	t.Run("CreatedSuccessfullyAtUpperAndLowerBounds", func(t *testing.T) {
		lowerCap, lowerErr := NewCapacity(0.0)
		upperCap, upperErr := NewCapacity(1.0)

		assert.NilError(t, lowerErr)
		assert.Equal(t, 0.0, lowerCap.cap)
		assert.NilError(t, upperErr)
		assert.Equal(t, 1.0, upperCap.cap)
	})

	t.Run("ErrorsIfNegative", func(t *testing.T) {
		cap, err := NewCapacity(-0.1)

		assert.Equal(t, 0.0, cap.Value())
		assert.Assert(t, testutils.ErrorIs(err, ErrInvalidCapacity))
		assert.ErrorContains(t, err, ": -0.1")
	})

	t.Run("ErrorsIfGreaterThanOne", func(t *testing.T) {
		cap, err := NewCapacity(1.1)

		assert.Equal(t, 0.0, cap.Value())
		assert.Assert(t, testutils.ErrorIs(err, ErrInvalidCapacity))
		assert.ErrorContains(t, err, ": 1.1")
	})
}

func TestCapacityEquals(t *testing.T) {
	assert.Equal(t, Capacity{0.5}, Capacity{0.5})
	assert.Assert(t, Capacity{0.5}.Equal(Capacity{1.0}) == false)
}
