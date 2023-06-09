//go:build small_tests || all_tests

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"github.com/spf13/cobra"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type CommandTestFixture struct {
	Cmd    *cobra.Command
	Stdout *strings.Builder
	Stderr *strings.Builder
}

func NewCommandTestFixture(cmd *cobra.Command) (f *CommandTestFixture) {
	f = &CommandTestFixture{
		Cmd: cmd, Stdout: &strings.Builder{}, Stderr: &strings.Builder{},
	}
	cmd.SetIn(bytes.NewReader([]byte{}))
	cmd.SetOut(f.Stdout)
	cmd.SetErr(f.Stderr)
	cmd.SetArgs([]string{})
	return
}

func (f *CommandTestFixture) ExecuteAndAssertStdoutContains(
	t *testing.T, expectedOutput string,
) {
	t.Helper()

	err := f.Cmd.Execute()

	assert.NilError(t, err)
	assert.Equal(t, "", f.Stderr.String(), "stderr should be empty")
	assert.Assert(t, is.Contains(f.Stdout.String(), expectedOutput))
}

func (f *CommandTestFixture) ExecuteAndAssertErrorContains(
	t *testing.T, expectedErrMsg string,
) (err error) {
	t.Helper()

	err = f.Cmd.Execute()

	assert.Equal(t, "", f.Stdout.String(), "stdout should be empty")
	assert.ErrorContains(t, err, expectedErrMsg)
	assert.Equal(t, fmt.Sprintf("Error: %s\n", err), f.Stderr.String())
	return
}

func (f *CommandTestFixture) AssertFailsIfRequiredFlagMissing(
	t *testing.T, flagName string, argv []string,
) {
	t.Helper()
	f.Cmd.SetArgs(argv)

	err := f.Cmd.Execute()

	expectedErr := "required flag(s) \"" + flagName + "\" not set"
	assert.ErrorContains(t, err, expectedErr)
}

func (f *CommandTestFixture) AssertReturnsLambdaError(
	t *testing.T, lambda *TestEListManFunc, expectedErr string,
) {
	t.Helper()
	lambda.InvokeError = fmt.Errorf("%w: invoke failed", ops.ErrExternal)

	err := f.ExecuteAndAssertErrorContains(t, expectedErr)

	assert.ErrorContains(t, err, "invoke failed")
	assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
}

type TestCloudFormationClient struct {
	DescribeStacksInput  *cloudformation.DescribeStacksInput
	DescribeStacksOutput *cloudformation.DescribeStacksOutput
	DescribeStacksError  error
}

func NewTestCloudFormationClient() *TestCloudFormationClient {
	return &TestCloudFormationClient{
		DescribeStacksOutput: &cloudformation.DescribeStacksOutput{
			Stacks: []cftypes.Stack{TestStack},
		},
	}
}

func (cfc *TestCloudFormationClient) DescribeStacks(
	_ context.Context,
	input *cloudformation.DescribeStacksInput,
	_ ...func(*cloudformation.Options),
) (*cloudformation.DescribeStacksOutput, error) {
	cfc.DescribeStacksInput = input
	return cfc.DescribeStacksOutput, cfc.DescribeStacksError
}
