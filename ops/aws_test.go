package ops

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/aws/smithy-go"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestAwsError(t *testing.T) {
	t.Run("DoesNotWrapIfNotAPIError", func(t *testing.T) {
		err := AwsError("test prefix", errors.New("Not an APIError"))

		assert.Error(t, err, "test prefix: Not an APIError")
		assert.Assert(t, testutils.ErrorIsNot(err, ErrExternal))
	})

	t.Run("DoesNotWrapIfNotServerError", func(t *testing.T) {
		apiErr := &smithy.GenericAPIError{
			Message: "Not a server error", Fault: smithy.FaultClient,
		}

		err := AwsError("test prefix", apiErr)

		assert.Error(t, err, "test prefix: api error : Not a server error")
		assert.Assert(t, testutils.ErrorIsNot(err, ErrExternal))
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

func TestLoadDefaultAwsConfig(t *testing.T) {
	t.Run("SucceedsIfValidConfigIsAvailable", func(t *testing.T) {
		_, err := LoadDefaultAwsConfig()

		assert.NilError(t, err)
	})

	t.Run("FailsOnInvalidConfig", func(t *testing.T) {
		// Simulate an invalid config by setting a deliberately bogus
		// environment variable value. This one is known to accept only "true,"
		// "false," or "auto."
		const varName = "AWS_ENABLE_ENDPOINT_DISCOVERY"
		orig := os.Getenv(varName)
		os.Setenv(varName, "bogus")
		defer os.Setenv(varName, orig)

		_, err := LoadDefaultAwsConfig()

		const expectedErrMsg = "failed to load AWS config: " +
			"invalid value for environment variable, " + varName +
			"=bogus, need true, false or auto"
		assert.Error(t, err, expectedErrMsg)
	})
}
