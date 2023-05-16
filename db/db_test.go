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

var testSubscribers []*Subscriber = []*Subscriber{
	{"foo@test.com", testUid, SubscriberVerified, testTimestamp},
	{"quux@test.com", testUid, SubscriberPending, testTimestamp},
	{"bar@test.com", testUid, SubscriberVerified, testTimestamp},
	{"xyzzy@test.com", testUid, SubscriberPending, testTimestamp},
	{"baz@test.com", testUid, SubscriberVerified, testTimestamp},
	{"plugh@test.com", testUid, SubscriberPending, testTimestamp},
}

var testPendingSubscribers []*Subscriber = []*Subscriber{
	testSubscribers[1],
	testSubscribers[3],
	testSubscribers[5],
}

var testVerifiedSubscribers []*Subscriber = []*Subscriber{
	testSubscribers[0],
	testSubscribers[2],
	testSubscribers[4],
}
