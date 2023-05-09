//go:build medium_tests || contract_tests || all_tests

package email

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestSesSuppressor(t *testing.T) {
	cfg, err := testutils.LoadDefaultAwsConfig()
	assert.NilError(t, err)

	logs := testutils.Logs{}
	suppressor := SesSuppressor{sesv2.NewFromConfig(cfg), logs.NewLogger()}
	email := testutils.RandomEmail(4, "elistman-test.com")
	ctx := context.Background()

	assert.Assert(t, !suppressor.IsSuppressed(ctx, email))

	err = suppressor.Suppress(ctx, email)
	assert.NilError(t, err)

	assert.Assert(t, suppressor.IsSuppressed(ctx, email))

	err = suppressor.Unsuppress(ctx, email)
	assert.NilError(t, err)

	assert.Assert(t, !suppressor.IsSuppressed(ctx, email))

	const expectedEmptyLogs = ""
	assert.Equal(t, expectedEmptyLogs, logs.Logs())
}