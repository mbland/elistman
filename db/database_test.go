//go:build small_tests || all_tests

package db

import (
	"fmt"
	"testing"

	"github.com/mbland/elistman/testdata"
	"gotest.tools/assert"
)

func TestSubscriber(t *testing.T) {
	t.Run("EmitsExpectedString", func(t *testing.T) {
		sub := &Subscriber{
			Email:     testdata.TestEmail,
			Uid:       testdata.TestUid,
			Status:    SubscriberPending,
			Timestamp: testdata.TestTimestamp}

		expected := fmt.Sprintf(
			"Email: %s, Uid: %s, Status: %s, Timestamp: %s",
			testdata.TestEmail,
			testdata.TestUid,
			string(SubscriberPending),
			testdata.TestTimeStr,
		)
		assert.Equal(t, expected, sub.String())
	})
}
