//go:build small_tests || all_tests

package cmd

import (
	"testing"

	"gotest.tools/assert"
)

// See the comment for TestLoadDefaultAwsConfig/SucceedsIfValidConfigIsAvailable
// for an explanation of why it's good to label this a small test, even though
// it's technically medium.
func TestAwsFactoryFunctions(t *testing.T) {
	assert.Assert(t, NewDynamoDb("imaginary-test-table") != nil)
	assert.Assert(t, NewLambdaClient() != nil)
}
