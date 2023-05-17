//go:build small_tests || medium_tests || all_tests

package email

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	tu "github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

const testUnsubEmail = "unsubscribe@foo.com"
const testApiBaseUrl = "https://foo.com/email"
const testUid = "00000000-1111-2222-3333-444444444444"

type TestSuppressor struct {
	checkedEmail       string
	isSuppressedResult bool
	isSuppressedErr    error
	suppressedEmail    string
	suppressErr        error
	unsuppressedEmail  string
	unsuppressErr      error
}

func (ts *TestSuppressor) IsSuppressed(
	ctx context.Context, email string,
) (bool, error) {
	ts.checkedEmail = email
	return ts.isSuppressedResult, ts.isSuppressedErr
}

func (ts *TestSuppressor) Suppress(ctx context.Context, email string) error {
	ts.suppressedEmail = email
	return ts.suppressErr
}

func (ts *TestSuppressor) Unsuppress(ctx context.Context, email string) error {
	ts.unsuppressedEmail = email
	return ts.unsuppressErr
}

func TestEmitPreviewMessageFromJson(t *testing.T) {

	t.Run("Succeeds", func(t *testing.T) {
		input := strings.NewReader(ExampleMessageJson)
		output := &strings.Builder{}

		err := EmitPreviewMessageFromJson(input, output)

		assert.NilError(t, err)
		msg, _, pr := tu.ParseMultipartMessageAndBoundary(t, output.String())
		assert.Assert(t, msg != nil)
		textPart := tu.GetNextPartContent(t, pr, "text/plain")
		assert.Assert(t, textPart != "")
		htmlPart := tu.GetNextPartContent(t, pr, "text/html")
		assert.Assert(t, htmlPart != "")
	})

	t.Run("FailsIfInputRaisesError", func(t *testing.T) {
		testErr := errors.New("simulated I/O error")
		input := iotest.ErrReader(testErr)
		output := &strings.Builder{}

		err := EmitPreviewMessageFromJson(input, output)

		assert.Assert(t, tu.ErrorIs(err, testErr))
	})

	t.Run("FailsIfOutputRaisesError", func(t *testing.T) {
		input := strings.NewReader(ExampleMessageJson)
		output := &tu.ErrWriter{
			Buf:     &strings.Builder{},
			ErrorOn: "Hello, World!",
			Err:     errors.New("simulated I/O error"),
		}

		err := EmitPreviewMessageFromJson(input, output)

		assert.Assert(t, tu.ErrorIs(err, output.Err))
	})
}
