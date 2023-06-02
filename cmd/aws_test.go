//go:build small_tests || all_tests

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
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
		cfc.DescribeStacksOutput.Stacks = []cftypes.Stack{}

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

func TestNewLambda(t *testing.T) {
	setup := func() (
		ctx context.Context,
		cfc *TestCloudFormationClient,
		tlc *TestLambdaClient,
	) {
		ctx = context.Background()
		cfc = NewTestCloudFormationClient()
		tlc = NewTestLambdaClient()
		return
	}

	t.Run("Succeeds", func(t *testing.T) {
		ctx, cfc, tlc := setup()

		l, err := NewLambda(ctx, cfc, tlc, TestStackName)

		assert.NilError(t, err)
		assert.Equal(t, TestFunctionArn, l.Arn)
		assert.Assert(t, l.Client == tlc)
	})

	t.Run("FailsIfCannotGetLambdaArn", func(t *testing.T) {
		ctx, cfc, tlc := setup()
		cfc.DescribeStacksError = testutils.AwsServerError("test error")

		l, err := NewLambda(ctx, cfc, tlc, TestStackName)

		assert.Assert(t, is.Nil(l))
		assert.ErrorContains(t, err, "could not create Lambda: ")
		assert.ErrorContains(t, err, "test error")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})
}

func TestLambdaInvoke(t *testing.T) {
	type lambdaRequest struct {
		Message string
	}

	type lambdaResponse struct {
		Message string
	}

	type unmarshalable struct {
		Foo func()
	}

	setup := func() (
		ctx context.Context,
		tlc *TestLambdaClient,
		l *Lambda,
		req *lambdaRequest,
		res *lambdaResponse,
	) {
		ctx = context.Background()
		tlc = NewTestLambdaClient()
		l = &Lambda{Client: tlc, Arn: TestFunctionArn}
		req = &lambdaRequest{Message: "Hello, World!"}
		res = &lambdaResponse{}

		tlc.InvokeOutput.StatusCode = http.StatusOK
		tlc.InvokeOutput.Payload, _ = json.Marshal(
			&lambdaResponse{Message: req.Message},
		)
		return
	}

	t.Run("Succeeds", func(t *testing.T) {
		ctx, _, l, req, res := setup()

		err := l.Invoke(ctx, req, res)

		assert.NilError(t, err)
		expectedResponse := &lambdaResponse{Message: "Hello, World!"}
		assert.DeepEqual(t, expectedResponse, res)
	})

	t.Run("FailsIfCannotMarshalRequest", func(t *testing.T) {
		ctx, _, l, _, res := setup()

		err := l.Invoke(ctx, &unmarshalable{}, res)

		assert.ErrorContains(t, err, "failed to marshal Lambda request payload")
	})

	t.Run("FailsIfCannotInvokeLambda", func(t *testing.T) {
		ctx, tlc, l, req, res := setup()
		tlc.InvokeError = testutils.AwsServerError("test error")

		err := l.Invoke(ctx, req, res)

		assert.ErrorContains(t, err, "error invoking Lambda function: ")
		assert.ErrorContains(t, err, "test error")
		assert.Assert(t, testutils.ErrorIs(err, ops.ErrExternal))
	})

	t.Run("FailsIfStatusCodeIsNotHttp200", func(t *testing.T) {
		ctx, tlc, l, req, res := setup()
		tlc.InvokeOutput.StatusCode = http.StatusBadRequest

		err := l.Invoke(ctx, req, res)

		expectedErr := "received non-200 response from Lambda invocation: " +
			http.StatusText(http.StatusBadRequest)
		assert.ErrorContains(t, err, expectedErr)
	})

	t.Run("FailsIfLambdaReturnedError", func(t *testing.T) {
		ctx, tlc, l, req, res := setup()
		tlc.InvokeOutput.FunctionError = aws.String("Lambda error")
		tlc.InvokeOutput.Payload = []byte("something went wrong")

		err := l.Invoke(ctx, req, res)

		const expectedErr = "error executing Lambda function: " +
			"Lambda error: something went wrong"
		assert.ErrorContains(t, err, expectedErr)
	})

	t.Run("FailsIfCannotUnmarshalPayload", func(t *testing.T) {
		ctx, tlc, l, req, res := setup()
		tlc.InvokeOutput.Payload = []byte("bogus, invalid payload")

		err := l.Invoke(ctx, req, res)

		const expectedErr = "failed to unmarshal Lambda response payload: "
		assert.ErrorContains(t, err, expectedErr)
		assert.ErrorContains(t, err, "bogus, invalid payload")
	})
}
