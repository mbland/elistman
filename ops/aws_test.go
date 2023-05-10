package ops

import (
	"errors"
	"testing"

	"github.com/aws/smithy-go"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestAwsError(t *testing.T) {
	t.Run("ReturnsOriginalIfNotAPIError", func(t *testing.T) {
		err := errors.New("Not an APIError")

		result := AwsError(err)

		assert.Equal(t, err, result)
		assert.Assert(t, !errors.Is(result, ErrExternal))
	})

	t.Run("ReturnsOriginalIfNotServerError", func(t *testing.T) {
		err := &smithy.GenericAPIError{Fault: smithy.FaultClient}

		result := AwsError(err)

		assert.Equal(t, err, result)
		assert.Assert(t, !errors.Is(result, ErrExternal))
	})

	t.Run("WrapsServerErrorWithErrExternal", func(t *testing.T) {
		err := &smithy.GenericAPIError{Fault: smithy.FaultServer}

		result := AwsError(err)

		assert.Assert(t, err != result)
		assert.Assert(t, testutils.ErrorIs(result, ErrExternal))
	})
}
