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
	"github.com/mbland/elistman/events"
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

func (f *CommandTestFixture) AssertCommandLineEventMatches(
	t *testing.T,
	lambda *TestEListManFunc,
	stackName string,
	expectedEvent *events.CommandLineEvent,
) {
	t.Helper()

	assert.Equal(t, stackName, lambda.StackName, "stack names should match")
	const cliEventMsg = "lambda request should be an *events.CommandLineEvent"
	req, isCliEvent := lambda.InvokeReq.(*events.CommandLineEvent)
	assert.Assert(t, isCliEvent == true, cliEventMsg)
	assert.DeepEqual(t, expectedEvent, req)
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
