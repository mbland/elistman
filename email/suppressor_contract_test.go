//go:build medium_tests || contract_tests || all_tests

package email

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestSesSuppressor(t *testing.T) {
	cfg, err := ops.LoadDefaultAwsConfig()
	assert.NilError(t, err)

	logs := testutils.Logs{}
	suppressor := SesSuppressor{sesv2.NewFromConfig(cfg)}
	email := testutils.RandomEmail(4, "elistman-test.com")
	ctx := context.Background()

	verdict, err := suppressor.IsSuppressed(ctx, email)
	assert.NilError(t, err)
	assert.Assert(t, verdict == false)

	err = suppressor.Suppress(ctx, email, ops.RemoveReasonBounce)
	assert.NilError(t, err)

	verdict, err = suppressor.IsSuppressed(ctx, email)
	assert.NilError(t, err)
	assert.Assert(t, verdict == true)

	// Suppressing an already suppressed address should be OK.
	err = suppressor.Suppress(ctx, email, ops.RemoveReasonBounce)
	assert.NilError(t, err)

	err = suppressor.Unsuppress(ctx, email)
	assert.NilError(t, err)

	verdict, err = suppressor.IsSuppressed(ctx, email)
	assert.NilError(t, err)
	assert.Assert(t, verdict == false)

	// Unsuppressing nonexistent address should be OK.
	err = suppressor.Unsuppress(ctx, email)
	assert.NilError(t, err)

	const expectedEmptyLogs = ""
	assert.Equal(t, expectedEmptyLogs, logs.Logs())
}
