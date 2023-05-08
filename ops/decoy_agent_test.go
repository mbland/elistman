//go:build small_tests || all_tests

package ops

import (
	"context"
	"testing"

	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestDecoyAgent(t *testing.T) {
	da := DecoyAgent{}
	ctx := context.Background()

	result, err := da.Subscribe(ctx, "foo@bar.com")
	assert.Equal(t, VerifyLinkSent, result)
	assert.NilError(t, err)

	result, err = da.Verify(ctx, "foo@bar.com", testutils.TestUid)
	assert.Equal(t, Subscribed, result)
	assert.NilError(t, err)

	result, err = da.Unsubscribe(ctx, "foo@bar.com", testutils.TestUid)
	assert.Equal(t, Unsubscribed, result)
	assert.NilError(t, err)

	err = da.Remove(ctx, "foo@bar.com")
	assert.NilError(t, err)

	err = da.Restore(ctx, "foo@bar.com")
	assert.NilError(t, err)
}
