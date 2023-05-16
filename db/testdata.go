//go:build small_tests || medium_tests || contract_tests || all_tests

package db

import (
	"time"

	"github.com/google/uuid"
	"github.com/mbland/elistman/testutils"
)

const TestEmail = testutils.TestEmail
const TestTimeStr = testutils.TestTimeStr

var TestUid uuid.UUID = testutils.TestUid

var TestTimestamp time.Time = testutils.TestTimestamp

var TestSubscribers []*Subscriber = []*Subscriber{
	{
		Email:     "foo@test.com",
		Uid:       uuid.MustParse("00000000-0000-0000-0000-000000000000"),
		Status:    SubscriberVerified,
		Timestamp: TestTimestamp,
	},
	{
		Email:     "quux@test.com",
		Uid:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Status:    SubscriberPending,
		Timestamp: TestTimestamp.Add(time.Hour * 24),
	},
	{
		Email:     "bar@test.com",
		Uid:       uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Status:    SubscriberVerified,
		Timestamp: TestTimestamp.Add(time.Hour * 48),
	},
	{
		Email:     "xyzzy@test.com",
		Uid:       uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		Status:    SubscriberPending,
		Timestamp: TestTimestamp.Add(time.Hour * 72),
	},
	{
		Email:     "baz@test.com",
		Uid:       uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		Status:    SubscriberVerified,
		Timestamp: TestTimestamp.Add(time.Hour * 96),
	},
	{
		Email:     "plugh@test.com",
		Uid:       uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Status:    SubscriberPending,
		Timestamp: TestTimestamp.Add(time.Hour * 120),
	},
}

var TestPendingSubscribers []*Subscriber = []*Subscriber{
	TestSubscribers[1],
	TestSubscribers[3],
	TestSubscribers[5],
}

var TestVerifiedSubscribers []*Subscriber = []*Subscriber{
	TestSubscribers[0],
	TestSubscribers[2],
	TestSubscribers[4],
}
