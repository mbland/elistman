//go:build small_tests || all_tests

package db

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"gotest.tools/assert"
)

const testEmail = "foo@bar.com"
const testTimeStr = "Fri, 18 Sep 1970 12:45:00 +0000"

var testUid uuid.UUID = uuid.MustParse("00000000-1111-2222-3333-444444444444")

var testTimestamp time.Time

func init() {
	var err error
	testTimestamp, err = time.Parse(TimestampFormat, testTimeStr)

	if err != nil {
		panic("failed to parse testTimestamp: " + err.Error())
	}
}

func TestSubscriber(t *testing.T) {
	t.Run("EmitsExpectedString", func(t *testing.T) {
		sub := &Subscriber{testEmail, testUid, SubscriberPending, testTimestamp}

		const strFmt = "Email: %s, Uid: %s, Status: %s, Timestamp: %s"
		expected := fmt.Sprintf(
			strFmt, testEmail, testUid, string(SubscriberPending), testTimeStr,
		)
		assert.Equal(t, expected, sub.String())
	})
}
