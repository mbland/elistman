//go:build small_tests || contract_tests || all_tests

package db

import (
	"fmt"
	"testing"

	"github.com/mbland/elistman/types"
	"gotest.tools/assert"
)

func TestSubscriber(t *testing.T) {
	t.Run("EmitsExpectedString", func(t *testing.T) {
		sub := &types.Subscriber{
			Email:     testEmail,
			Uid:       testUid,
			Status:    types.SubscriberPending,
			Timestamp: testTimestamp,
		}

		expected := fmt.Sprintf(
			"Email: %s, Uid: %s, Status: %s, Timestamp: %s",
			testEmail,
			testUid,
			string(types.SubscriberPending),
			testTimeStr,
		)
		assert.Equal(t, expected, sub.String())
	})
}
