//go:build small_tests || all_tests

package email

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
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
		testSesV2.getSupDestError = notFoundException()

		verdict, err := suppressor.IsSuppressed(ctx, "foo@bar.com")

		assert.NilError(t, err)
		assert.Assert(t, verdict == false)
	})

	t.Run("ReturnsErrorIfUnexpectedFailure", func(t *testing.T) {
		testSesV2, suppressor, ctx := setup()
		testSesV2.getSupDestError = testutils.AwsServerError("not a 404")

		verdict, err := suppressor.IsSuppressed(ctx, "foo@bar.com")

		assert.Assert(t, verdict == false)
		const expectedErr = "unexpected error " +
			"while checking if foo@bar.com suppressed: " +
			"external error: api error : not a 404"
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

	const reasonBounce = ops.RemoveReasonBounce
	const reasonComplaint = ops.RemoveReasonComplaint

	t.Run("SucceedsForBounce", func(t *testing.T) {
		testSesV2, suppressor, ctx := setup()

		err := suppressor.Suppress(ctx, "foo@bar.com", reasonBounce)

		assert.NilError(t, err)
		actualReq := testSesV2.putSupDestInput
		assert.Equal(t, "foo@bar.com", aws.ToString(actualReq.EmailAddress))
		assert.Equal(t, types.SuppressionListReasonBounce, actualReq.Reason)
	})

	t.Run("SucceedsForComplaint", func(t *testing.T) {
		testSesV2, suppressor, ctx := setup()

		err := suppressor.Suppress(ctx, "foo@bar.com", reasonComplaint)

		assert.NilError(t, err)
		actualReason := testSesV2.putSupDestInput.Reason
		assert.Equal(t, types.SuppressionListReasonComplaint, actualReason)
	})

	t.Run("ReturnsAnError", func(t *testing.T) {
		testSesV2, suppressor, ctx := setup()
		testSesV2.putSupDestError = testutils.AwsServerError("testing")

		err := suppressor.Suppress(ctx, "foo@bar.com", reasonComplaint)

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
		testSesV2.deleteSupDestError = notFoundException()

		err := suppressor.Unsuppress(ctx, "foo@bar.com")

		assert.NilError(t, err)
	})

	t.Run("ReturnsAnError", func(t *testing.T) {
		testSesV2, suppressor, ctx := setup()
		testSesV2.deleteSupDestError = testutils.AwsServerError("testing")

		err := suppressor.Unsuppress(ctx, "foo@bar.com")

		const expectedErr = "failed to unsuppress foo@bar.com: "
		assert.ErrorContains(t, err, expectedErr)
		assert.ErrorContains(t, err, "testing")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})
}
