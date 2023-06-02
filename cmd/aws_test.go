//go:build small_tests || all_tests

package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

// See the comment for TestLoadDefaultAwsConfig/SucceedsIfValidConfigIsAvailable
// for an explanation of why it's good to label this a small test, even though
// it's technically medium.
func TestAwsFactoryFunctions(t *testing.T) {
	assert.Assert(t, NewDynamoDb("imaginary-test-table") != nil)
	assert.Assert(t, NewLambdaClient() != nil)
	assert.Assert(t, NewCloudFormationClient() != nil)
}

func TestGetLambdaArn(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		cfc := NewTestCloudFormationClient()

		arn, err := GetLambdaArn(context.Background(), cfc, TestStackName)

		assert.NilError(t, err)
		assert.Equal(t, TestFunctionArn, arn)
	})

	t.Run("FailsIfDescribeStacksFails", func(t *testing.T) {
		cfc := NewTestCloudFormationClient()
		cfc.DescribeStacksError = testutils.AwsServerError("test error")

		_, err := GetLambdaArn(context.Background(), cfc, TestStackName)

		expectedMsg := "failed to get Lambda ARN for " + TestStackName
		assert.ErrorContains(t, err, expectedMsg)
		assert.ErrorContains(t, err, "test error")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})

	t.Run("FailsIfStackNotFound", func(t *testing.T) {
		cfc := NewTestCloudFormationClient()
		cfc.DescribeStacksOutput.Stacks = []types.Stack{}

		_, err := GetLambdaArn(context.Background(), cfc, TestStackName)

		assert.ErrorContains(t, err, "stack not found: "+TestStackName)
	})

	t.Run("FailsIfFunctionArnKeyMissing", func(t *testing.T) {
		cfc := NewTestCloudFormationClient()
		cfc.DescribeStacksOutput.Stacks[0].Outputs = []cftypes.Output{
			{
				OutputKey:   aws.String("NotTheExpectedKey"),
				OutputValue: aws.String(TestFunctionArn),
			},
		}

		_, err := GetLambdaArn(context.Background(), cfc, TestStackName)

		const errFmt = `stack "%s" doesn't contain output key "%s"`
		expectedMsg := fmt.Sprintf(errFmt, TestStackName, FunctionArnKey)
		assert.ErrorContains(t, err, expectedMsg)
	})
}
