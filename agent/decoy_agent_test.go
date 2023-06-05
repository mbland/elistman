//go:build small_tests || all_tests

package agent

import (
	"context"
	"testing"

	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/testdata"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestDecoyAgent(t *testing.T) {
	da := DecoyAgent{}
	ctx := context.Background()

	result, err := da.Subscribe(ctx, "foo@bar.com")
	assert.Equal(t, ops.VerifyLinkSent, result)
	assert.NilError(t, err)

	result, err = da.Verify(ctx, "foo@bar.com", testdata.TestUid)
	assert.Equal(t, ops.Subscribed, result)
	assert.NilError(t, err)

	result, err = da.Unsubscribe(ctx, "foo@bar.com", testdata.TestUid)
	assert.Equal(t, ops.Unsubscribed, result)
	assert.NilError(t, err)

	failure, err := da.Validate(ctx, "foo@bar.com")
	assert.Assert(t, is.Nil(failure))
	assert.NilError(t, err)

	err = da.Remove(ctx, "foo@bar.com")
	assert.NilError(t, err)

	err = da.Restore(ctx, "foo@bar.com")
	assert.NilError(t, err)

	numSent, err := da.Send(ctx, nil)
	assert.NilError(t, err)
	assert.Equal(t, 0, numSent)
}
