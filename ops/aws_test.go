package ops

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/smithy-go"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestAwsError(t *testing.T) {
	t.Run("DoesNotWrapIfNotAPIError", func(t *testing.T) {
		err := AwsError("test prefix", errors.New("Not an APIError"))

		assert.Error(t, err, "test prefix: Not an APIError")
		assert.Assert(t, !errors.Is(err, ErrExternal))
	})

	t.Run("DoesNotWrapIfNotServerError", func(t *testing.T) {
		apiErr := &smithy.GenericAPIError{
			Message: "Not a server error", Fault: smithy.FaultClient,
		}

		err := AwsError("test prefix", apiErr)

		assert.Error(t, err, "test prefix: api error : Not a server error")
		assert.Assert(t, !errors.Is(err, ErrExternal))
	})

	t.Run("WrapsServerErrorWithErrExternal", func(t *testing.T) {
		apiErr := &smithy.GenericAPIError{
			Message: "Definitely a server error", Fault: smithy.FaultServer,
		}

		err := AwsError("test prefix", apiErr)

		expected := fmt.Sprintf(
			"%s: test prefix: api error : Definitely a server error",
			ErrExternal,
		)
		assert.Error(t, err, expected)
		assert.Assert(t, testutils.ErrorIs(err, ErrExternal))
	})
}
