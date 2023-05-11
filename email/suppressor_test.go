package email

import (
	"context"
	"errors"
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

func TestIsSuppressed(t *testing.T) {
	setup := func() (
		*TestSesV2, *testutils.Logs, *SesSuppressor, context.Context,
	) {
		testSesV2 := &TestSesV2{}
		logs, logger := testutils.NewLogs()
		suppressor := &SesSuppressor{Client: testSesV2, Log: logger}
		return testSesV2, logs, suppressor, context.Background()
	}

	t.Run("ReturnsTrueIfSuppressed", func(t *testing.T) {
		_, logs, suppressor, ctx := setup()

		assert.Assert(t, suppressor.IsSuppressed(ctx, "foo@bar.com"))

		expectedEmptyLogs := ""
		assert.Equal(t, expectedEmptyLogs, logs.Logs())
	})

	t.Run("ReturnsFalse", func(t *testing.T) {
		t.Run("IfNotSuppressed", func(t *testing.T) {
			testSesV2, logs, suppressor, ctx := setup()
			// Wrap the following error to make sure the implementation is using
			// errors.As properly, versus a type assertion.
			testSesV2.getError = fmt.Errorf(
				"404: %w", &types.NotFoundException{},
			)

			assert.Assert(t, !suppressor.IsSuppressed(ctx, "foo@bar.com"))

			expectedEmptyLogs := ""
			assert.Equal(t, expectedEmptyLogs, logs.Logs())
		})

		t.Run("AndLogsErrorIfUnexpectedError", func(t *testing.T) {
			testSesV2, logs, suppressor, ctx := setup()
			testSesV2.getError = errors.New("not a 404")

			assert.Assert(t, !suppressor.IsSuppressed(ctx, "foo@bar.com"))

			const expectedLog = "unexpected error while checking if " +
				"foo@bar.com suppressed: not a 404"
			logs.AssertContains(t, expectedLog)
		})
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
