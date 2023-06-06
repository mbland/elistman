//go:build small_tests || all_tests

package cmd

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mbland/elistman/events"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

type errReader struct {
}

func (er *errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("test read error")
}

func TestReadLines(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		lines, err := readLines(strings.NewReader("foo\nbar\nbaz"))

		assert.NilError(t, err)
		assert.DeepEqual(t, []string{"foo", "bar", "baz"}, lines)
	})

	t.Run("ReturnsError", func(t *testing.T) {
		lines, err := readLines(&errReader{})

		assert.Equal(t, 0, len(lines))
		assert.Error(t, err, "test read error")
	})
}

func TestImportSuccess(t *testing.T) {
	t.Run("Singular", func(t *testing.T) {
		msg := importSuccessMessage(1, 1000)

		assert.Equal(t, "Successfully imported one address.\n", msg)
	})

	t.Run("Plural", func(t *testing.T) {
		msg := importSuccessMessage(100, 1000)

		assert.Equal(t, "Successfully imported 100 of 1000 addresses.\n", msg)
	})
}

func TestErrorIfImportFailures(t *testing.T) {
	t.Run("NilIfNoFailures", func(t *testing.T) {
		assert.NilError(t, errorIfImportFailures([]string{}))
	})

	t.Run("SingleFailure", func(t *testing.T) {
		failures := []string{"foo@test.com: failed"}

		err := errorIfImportFailures(failures)

		assert.Error(t, err, "failed to import foo@test.com: failed")
	})

	t.Run("MultipleFailures", func(t *testing.T) {
		failures := []string{
			"foo@test.com: failed",
			"bar@test.com: failed",
			"baz@test.com: failed",
		}

		err := errorIfImportFailures(failures)

		const expectedErr = "failed to import the following 3 addresses:\n" +
			"  foo@test.com: failed\n" +
			"  bar@test.com: failed\n" +
			"  baz@test.com: failed"
		assert.Error(t, err, expectedErr)
	})
}

func TestImport(t *testing.T) {
	addrs := []string{"foo@test.com", "bar@test.com", "baz@test.com"}

	setup := func() (f *CommandTestFixture, lambda *TestEListManFunc) {
		lambda = NewTestEListManFunc()
		f = NewCommandTestFixture(newImportCmd(lambda.GetFactoryFunc()))
		f.Cmd.SetIn(strings.NewReader(strings.Join(addrs, "\n")))
		f.Cmd.SetArgs([]string{"-s", TestStackName})
		return
	}

	t.Run("Succeeds", func(t *testing.T) {
		f, lambda := setup()
		lambda.InvokeResJson = []byte(`{"NumImported": 3}`)

		const expectedOut = "Successfully imported 3 of 3 addresses.\n"
		f.ExecuteAndAssertStdoutContains(t, expectedOut)

		assert.Assert(t, f.Cmd.SilenceUsage == true)
		assert.Equal(t, TestStackName, lambda.StackName)
		req, isCliEvent := lambda.InvokeReq.(*events.CommandLineEvent)
		assert.Assert(t, isCliEvent == true)
		expectedReq := &events.CommandLineEvent{
			EListManCommand: events.CommandLineImportEvent,
			Import:          &events.ImportEvent{Addresses: addrs},
		}
		assert.DeepEqual(t, expectedReq, req)
	})

	t.Run("FailsIfStackNameNotSpecified", func(t *testing.T) {
		f, _ := setup()
		f.Cmd.SetArgs([]string{})

		err := f.Cmd.Execute()

		const expectedErr = "required flag(s) \"" + FlagStackName + "\" not set"
		assert.ErrorContains(t, err, expectedErr)
	})

	t.Run("FailsIfCannotParseInput", func(t *testing.T) {
		f, _ := setup()
		f.Cmd.SetIn(&errReader{})

		const expectedErr = "failed to read email addresses from stdin: " +
			"test read error"
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})

	t.Run("FailsIfCreatingLambdaFails", func(t *testing.T) {
		f, lambda := setup()
		const errFmt = "%w: creating lambda failed"
		lambda.CreateFuncError = fmt.Errorf(errFmt, ops.ErrExternal)

		err := f.ExecuteAndAssertErrorContains(t, "creating lambda failed")

		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})

	t.Run("FailsIfInvokingLambdaFails", func(t *testing.T) {
		f, lambda := setup()
		lambda.InvokeError = fmt.Errorf("%w: invoke failed", ops.ErrExternal)

		err := f.ExecuteAndAssertErrorContains(t, "import failed: ")

		assert.ErrorContains(t, err, "invoke failed")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})

	t.Run("FailsIfFailedToImportAnyAddresses", func(t *testing.T) {
		f, lambda := setup()
		lambda.InvokeResJson = []byte(`{
			"NumImported": 1,
			"Failures": [
				"foo@text.com: first error",
				"baz@text.com: second error"
			]
		}`)

		err := f.Cmd.Execute()

		const expectedStdout = "Successfully imported one address.\n"
		const expectedErr = "failed to import the following 2 addresses:\n" +
			"  foo@text.com: first error\n" +
			"  baz@text.com: second error"
		assert.Equal(t, expectedStdout, f.Stdout.String())
		assert.ErrorContains(t, err, expectedErr)
	})
}
