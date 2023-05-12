package email

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

type TestSesV2 struct {
	getInput     *sesv2.GetSuppressedDestinationInput
	getOutput    *sesv2.GetSuppressedDestinationOutput
	getError     error
	putInput     *sesv2.PutSuppressedDestinationInput
	putOutput    *sesv2.PutSuppressedDestinationOutput
	putError     error
	deleteInput  *sesv2.DeleteSuppressedDestinationInput
	deleteOutput *sesv2.DeleteSuppressedDestinationOutput
	deleteError  error
}

func (ses *TestSesV2) GetSuppressedDestination(
	_ context.Context,
	input *sesv2.GetSuppressedDestinationInput,
	_ ...func(*sesv2.Options),
) (*sesv2.GetSuppressedDestinationOutput, error) {
	ses.getInput = input
	return ses.getOutput, ses.getError
}

func (ses *TestSesV2) PutSuppressedDestination(
	_ context.Context,
	input *sesv2.PutSuppressedDestinationInput,
	_ ...func(*sesv2.Options),
) (*sesv2.PutSuppressedDestinationOutput, error) {
	ses.putInput = input
	return ses.putOutput, ses.putError
}

func (ses *TestSesV2) DeleteSuppressedDestination(
	_ context.Context,
	input *sesv2.DeleteSuppressedDestinationInput,
	_ ...func(*sesv2.Options),
) (*sesv2.DeleteSuppressedDestinationOutput, error) {
	ses.deleteInput = input
	return ses.deleteOutput, ses.deleteError
}

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
