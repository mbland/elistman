//go:build small_tests || medium_tests || contract_tests || all_tests

package db

import (
	"time"

	"github.com/google/uuid"
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

type BogusDbStartKey struct{}

func (*BogusDbStartKey) isDbStartKey() {}
