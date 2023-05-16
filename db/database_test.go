//go:build small_tests || contract_tests || all_tests

package db

import (
	"fmt"
	"testing"

	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestSubscriber(t *testing.T) {
	t.Run("EmitsExpectedString", func(t *testing.T) {
		sub := &Subscriber{
			Email:     testutils.TestEmail,
			Uid:       testutils.TestUid,
			Status:    SubscriberPending,
			Timestamp: testutils.TestTimestamp}

		expected := fmt.Sprintf(
			"Email: %s, Uid: %s, Status: %s, Timestamp: %s",
			testutils.TestEmail,
			testutils.TestUid,
			string(SubscriberPending),
			testutils.TestTimeStr,
		)
		assert.Equal(t, expected, sub.String())
	})
}
