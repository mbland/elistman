//go:build small_tests || all_tests

package email

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func notFoundException() error {
	// Wrap the error to make sure the implementation is using errors.As
	// properly, versus a type assertion.
	return fmt.Errorf("404: %w", &types.NotFoundException{})
}

func TestIsSuppressed(t *testing.T) {
	setup := func() (*TestSesV2, *SesSuppressor, context.Context) {
		testSesV2 := &TestSesV2{}
		suppressor := &SesSuppressor{Client: testSesV2}
		return testSesV2, suppressor, context.Background()
	}

	t.Run("ReturnsTrueIfSuppressed", func(t *testing.T) {
		_, suppressor, ctx := setup()

		verdict, err := suppressor.IsSuppressed(ctx, "foo@bar.com")

		assert.NilError(t, err)
		assert.Assert(t, verdict == true)
	})

	t.Run("ReturnsFalseIfNotSuppressed", func(t *testing.T) {
		testSesV2, suppressor, ctx := setup()
		testSesV2.getError = notFoundException()

		verdict, err := suppressor.IsSuppressed(ctx, "foo@bar.com")

		assert.NilError(t, err)
		assert.Assert(t, verdict == false)
	})

	t.Run("ReturnsErrorIfUnexpectedFailure", func(t *testing.T) {
		testSesV2, suppressor, ctx := setup()
		testSesV2.getError = testutils.AwsServerError("not a 404")

		verdict, err := suppressor.IsSuppressed(ctx, "foo@bar.com")

		assert.Assert(t, verdict == false)
		const expectedErr = "external error: " +
			"unexpected error while checking if foo@bar.com suppressed: " +
			"api error : not a 404"
		assert.ErrorContains(t, err, expectedErr)
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})
}

func TestSuppress(t *testing.T) {
	setup := func() (*TestSesV2, *SesSuppressor, context.Context) {
		testSesV2 := &TestSesV2{}
		suppressor := &SesSuppressor{Client: testSesV2}
		return testSesV2, suppressor, context.Background()
	}

	t.Run("Succeeds", func(t *testing.T) {
		_, suppressor, ctx := setup()

		err := suppressor.Suppress(ctx, "foo@bar.com")

		assert.NilError(t, err)
	})

	t.Run("ReturnsAnError", func(t *testing.T) {
		testSesV2, suppressor, ctx := setup()
		testSesV2.putError = testutils.AwsServerError("testing")

		err := suppressor.Suppress(ctx, "foo@bar.com")

		assert.ErrorContains(t, err, "failed to suppress foo@bar.com: ")
		assert.ErrorContains(t, err, "testing")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})
}

func TestUnsuppress(t *testing.T) {
	setup := func() (*TestSesV2, *SesSuppressor, context.Context) {
		testSesV2 := &TestSesV2{}
		suppressor := &SesSuppressor{Client: testSesV2}
		return testSesV2, suppressor, context.Background()
	}

	t.Run("Succeeds", func(t *testing.T) {
		_, suppressor, ctx := setup()

		err := suppressor.Unsuppress(ctx, "foo@bar.com")

		assert.NilError(t, err)
	})

	t.Run("SucceedsEvenIfUserIsNotSuppressed", func(t *testing.T) {
		testSesV2, suppressor, ctx := setup()
		testSesV2.deleteError = notFoundException()

		err := suppressor.Unsuppress(ctx, "foo@bar.com")

		assert.NilError(t, err)
	})

	t.Run("ReturnsAnError", func(t *testing.T) {
		testSesV2, suppressor, ctx := setup()
		testSesV2.deleteError = testutils.AwsServerError("testing")

		err := suppressor.Unsuppress(ctx, "foo@bar.com")

		const expectedErr = "failed to unsuppress foo@bar.com: "
		assert.ErrorContains(t, err, expectedErr)
		assert.ErrorContains(t, err, "testing")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})
}
