//go:build small_tests || contract_tests || all_tests

package db

import (
	"fmt"
	"testing"

	"gotest.tools/assert"
)

func TestSubscriber(t *testing.T) {
	t.Run("EmitsExpectedString", func(t *testing.T) {
		sub := &Subscriber{TestEmail, TestUid, SubscriberPending, TestTimestamp}

		const strFmt = "Email: %s, Uid: %s, Status: %s, Timestamp: %s"
		expected := fmt.Sprintf(
			strFmt, TestEmail, TestUid, string(SubscriberPending), TestTimeStr,
		)
		assert.Equal(t, expected, sub.String())
	})
}
