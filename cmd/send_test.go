//go:build small_tests || all_tests

package cmd

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/mbland/elistman/email"
	tu "github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type TestLambdaClient struct {
	InvokeInput  *lambda.InvokeInput
	InvokeOutput *lambda.InvokeOutput
	InvokeError  error
}

func NewTestLambdaClient() *TestLambdaClient {
	return &TestLambdaClient{InvokeOutput: &lambda.InvokeOutput{}}
}

func (tlc *TestLambdaClient) Invoke(
	_ context.Context, input *lambda.InvokeInput, _ ...func(*lambda.Options),
) (*lambda.InvokeOutput, error) {
	tlc.InvokeInput = input
	return tlc.InvokeOutput, tlc.InvokeError
}

func TestMustMarshal(t *testing.T) {
	type Marshalable struct {
		Foo string
	}

	type Unmarshalable struct {
		Foo func()
	}

	t.Run("Succeeds", func(t *testing.T) {
		payload := mustMarshal(&Marshalable{Foo: "bar"}, "this shouldn't panic")

		assert.Equal(t, `{"Foo":"bar"}`, string(payload))
	})

	t.Run("FailsIfUnsupported", func(t *testing.T) {
		defer tu.ExpectPanic(t, "this should totally panic")

		mustMarshal(&Unmarshalable{Foo: func() {}}, "this should totally panic")
	})
}

func TestSend(t *testing.T) {
	setup := func() (
		f *CommandTestFixture,
		cfc *TestCloudFormationClient,
		tlc *TestLambdaClient,
	) {
		cfc = NewTestCloudFormationClient()

		tlc = NewTestLambdaClient()
		tlc.InvokeOutput.StatusCode = http.StatusOK
		tlc.InvokeOutput.Payload = []byte(`{"Success": true, "NumSent": 27}`)

		f = NewCommandTestFixture(
			newSendCmd(
				func() CloudFormationClient { return cfc },
				func() LambdaClient { return tlc },
			),
		)
		f.Cmd.SetIn(strings.NewReader(email.ExampleMessageJson))
		f.Cmd.SetArgs([]string{TestStackName})
		return
	}

	t.Run("Succeeds", func(t *testing.T) {
		f, _, tlc := setup()

		const expectedOut = "Sent the message successfully to 27 recipients.\n"
		f.ExecuteAndAssertStdoutContains(t, expectedOut)

		assert.Assert(t, f.Cmd.SilenceUsage == true)
		invokeFunctionName := tlc.InvokeInput.FunctionName
		tu.AssertAwsStringEqual(t, TestFunctionArn, invokeFunctionName)
		payload := bytes.NewReader(tlc.InvokeInput.Payload)
		actualMsg := email.MustParseMessageFromJson(payload)
		assert.DeepEqual(t, email.ExampleMessage, actualMsg)
	})

	t.Run("FailsIfCannotParseInput", func(t *testing.T) {
		f, _, _ := setup()
		f.Cmd.SetIn(strings.NewReader("not a message input"))

		const expectedErr = "failed to parse message input from JSON: "
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})

	t.Run("FailsIfGettingFunctionArnFails", func(t *testing.T) {
		f, cfc, _ := setup()
		cfc.DescribeStacksOutput.Stacks = []cftypes.Stack{}

		f.ExecuteAndAssertErrorContains(t, "stack not found: "+TestStackName)
	})

	t.Run("FailsIfCannotInvokeLambda", func(t *testing.T) {
		f, _, tlc := setup()
		tlc.InvokeError = errors.New("invoke failed")

		const expectedErr = "error invoking Lambda function: invoke failed"
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})

	t.Run("FailsIfStatusCodeIsNotHttp200", func(t *testing.T) {
		f, _, tlc := setup()
		tlc.InvokeOutput.StatusCode = http.StatusBadRequest

		expectedErr := "received non-200 response from Lambda invocation: " +
			http.StatusText(http.StatusBadRequest)
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})

	t.Run("FailsIfLambdaReturnedError", func(t *testing.T) {
		f, _, tlc := setup()
		tlc.InvokeOutput.FunctionError = aws.String("Lambda error")
		tlc.InvokeOutput.Payload = []byte("something went wrong")

		const expectedErr = "error executing Lambda function: " +
			"Lambda error: something went wrong"
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})

	t.Run("FailsIfCannotUnmarshalPayload", func(t *testing.T) {
		f, _, tlc := setup()
		tlc.InvokeOutput.Payload = []byte("bogus, invalid payload")

		const expectedErr = "failed to unmarshal Lambda response payload: "
		f.ExecuteAndAssertErrorContains(t, expectedErr)
		assert.Assert(
			t, is.Contains(f.Stderr.String(), "bogus, invalid payload"),
		)
	})

	t.Run("FailsIfSendingFailed", func(t *testing.T) {
		f, _, tlc := setup()
		tlc.InvokeOutput.Payload = []byte(
			`{"Success": false, "NumSent": 9, "Details": "test failure"}`,
		)

		const expectedErr = "sending failed after sending to 9 recipients: " +
			"test failure"
		f.ExecuteAndAssertErrorContains(t, expectedErr)
	})
}
