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
	{
		Email:     "foo@test.com",
		Uid:       testUid,
		Status:    SubscriberVerified,
		Timestamp: testTimestamp,
	},
	{
		Email:     "quux@test.com",
		Uid:       testUid,
		Status:    SubscriberPending,
		Timestamp: testTimestamp,
	},
	{
		Email:     "bar@test.com",
		Uid:       testUid,
		Status:    SubscriberVerified,
		Timestamp: testTimestamp,
	},
	{
		Email:     "xyzzy@test.com",
		Uid:       testUid,
		Status:    SubscriberPending,
		Timestamp: testTimestamp,
	},
	{
		Email:     "baz@test.com",
		Uid:       testUid,
		Status:    SubscriberVerified,
		Timestamp: testTimestamp,
	},
	{
		Email:     "plugh@test.com",
		Uid:       testUid,
		Status:    SubscriberPending,
		Timestamp: testTimestamp,
	},
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
