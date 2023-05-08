//go:build small_tests || medium_tests || contract_tests || all_tests

package db

import (
	"time"

	"github.com/google/uuid"
	"github.com/mbland/elistman/testutils"
)

const testEmail = testutils.TestEmail
const testTimeStr = testutils.TestTimeStr

var testUid uuid.UUID = testutils.TestUid

var testTimestamp time.Time = testutils.TestTimestamp

var testPendingSubscribers []*Subscriber = []*Subscriber{
	{"quux@test.com", testUid, SubscriberPending, testTimestamp},
	{"xyzzy@test.com", testUid, SubscriberPending, testTimestamp},
	{"plugh@test.com", testUid, SubscriberPending, testTimestamp},
}

var testVerifiedSubscribers []*Subscriber = []*Subscriber{
	{"foo@test.com", testUid, SubscriberVerified, testTimestamp},
	{"bar@test.com", testUid, SubscriberVerified, testTimestamp},
	{"baz@test.com", testUid, SubscriberVerified, testTimestamp},
}
