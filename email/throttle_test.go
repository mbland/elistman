//go:build small_tests || all_tests

package email

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/mbland/elistman/testdata"
	"github.com/mbland/elistman/testutils"
	"github.com/mbland/elistman/types"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type sesThrottleFixture struct {
	ctx           context.Context
	client        *TestSesV2
	quota         *sesv2types.SendQuota
	capacity      types.Capacity
	created       time.Time
	sleepDuration time.Duration
	sleep         func(time.Duration)
}

func newSesThrottleFixture() *sesThrottleFixture {
	capacity, _ := types.NewCapacity(0.75)
	f := &sesThrottleFixture{
		ctx:    context.Background(),
		client: &TestSesV2{},
		quota: &sesv2types.SendQuota{
			MaxSendRate:     25.0,
			Max24HourSend:   50000.0,
			SentLast24Hours: 25000.0,
		},
		capacity: capacity,
		created:  testdata.TestTimestamp,
	}
	f.client.getAccountOutput = &sesv2.GetAccountOutput{SendQuota: f.quota}
	f.sleep = func(sleepFor time.Duration) {
		f.sleepDuration = sleepFor
	}
	return f
}

func (f *sesThrottleFixture) NewSesThrottle() (*SesThrottle, error) {
	return NewSesThrottle(f.ctx, f.client, f.capacity, f.created, f.sleep)
}

func (f *sesThrottleFixture) NewSesThrottleFailOnErr(
	t *testing.T,
) *SesThrottle {
	t.Helper()

	throttle, err := f.NewSesThrottle()

	if err != nil {
		t.Fatalf("unexpected test setup error: %s", err)
	}
	return throttle
}

func TestNewSesThrottleIncludingRefresh(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		f := newSesThrottleFixture()

		throttle, err := f.NewSesThrottle()
		throttle.Sleep(time.Second)

		assert.NilError(t, err)
		assert.Assert(t, f.client.getAccountInput != nil)
		assert.Equal(t, f.client, throttle.Client)
		assert.Equal(t, f.created, throttle.Created)
		assert.Equal(t, time.Duration(time.Second/25), throttle.PauseInterval)
		assert.Assert(t, testutils.TimesEqual(time.Time{}, throttle.LastSend))
		assert.Equal(t, time.Second, f.sleepDuration)
		assert.Equal(t, int(f.quota.Max24HourSend), throttle.Max24HourSend)
		assert.Equal(t, int(f.quota.SentLast24Hours), throttle.SentLast24Hours)
		assert.Equal(t, f.capacity.Value(), throttle.MaxBulkCapacity.Value())
		assert.Equal(t, 37500, throttle.MaxBulkSendable)
	})

	t.Run("FailsIfRefreshFails", func(t *testing.T) {
		f := newSesThrottleFixture()
		f.client.getAccountError = errors.New("test error")

		throttle, err := f.NewSesThrottle()

		assert.Assert(t, is.Nil(throttle))
		assert.Error(t, err, "failed to get AWS account info: test error")
	})
}

func TestRefreshIfExpired(t *testing.T) {
	setup := func(t *testing.T) (*sesThrottleFixture, *SesThrottle) {
		f := newSesThrottleFixture()
		throttle := f.NewSesThrottleFailOnErr(t)
		f.client.getAccountOutput.SendQuota = &sesv2types.SendQuota{
			MaxSendRate:     f.quota.MaxSendRate * 2,
			Max24HourSend:   f.quota.Max24HourSend * 2,
			SentLast24Hours: f.quota.SentLast24Hours * 2,
		}
		return f, throttle
	}

	t.Run("DoesNothingIfNotExpired", func(t *testing.T) {
		f, throttle := setup(t)
		now := f.created.Add(time.Minute - time.Nanosecond)

		err := throttle.RefreshIfExpired(f.ctx, time.Minute, now)

		assert.NilError(t, err)
		assert.Equal(t, int(f.quota.Max24HourSend), throttle.Max24HourSend)
	})

	t.Run("RefreshesIfExpired", func(t *testing.T) {
		f, throttle := setup(t)
		now := f.created.Add(time.Minute)

		err := throttle.RefreshIfExpired(f.ctx, time.Minute, now)

		assert.NilError(t, err)
		assert.Equal(t, int(f.quota.Max24HourSend)*2, throttle.Max24HourSend)
	})
}

func TestBulkCapacityAvailable(t *testing.T) {
	setup := func(t *testing.T) (*sesThrottleFixture, *SesThrottle) {
		f := newSesThrottleFixture()
		return f, f.NewSesThrottleFailOnErr(t)
	}

	t.Run("Succeeds", func(t *testing.T) {
		_, throttle := setup(t)
		numToSend := throttle.MaxBulkSendable - throttle.SentLast24Hours

		err := throttle.BulkCapacityAvailable(numToSend)

		assert.NilError(t, err)
	})

	t.Run("ErrorsIfInsufficientCapacity", func(t *testing.T) {
		_, throttle := setup(t)
		numToSend := throttle.MaxBulkSendable - throttle.SentLast24Hours + 1

		err := throttle.BulkCapacityAvailable(numToSend)

		assert.Assert(t, testutils.ErrorIs(err, ErrBulkSendWouldExceedCapacity))
		const expectedFmt = "%d total send max, %s desired bulk capacity, " +
			"%d bulk sendable, %d sent last 24h, %d requested"
		expectedMsg := fmt.Sprintf(
			expectedFmt,
			throttle.Max24HourSend,
			throttle.MaxBulkCapacity,
			throttle.MaxBulkSendable,
			throttle.SentLast24Hours,
			numToSend,
		)
		assert.ErrorContains(t, err, expectedMsg)
	})
}

func TestPauseBeforeNextSend(t *testing.T) {
	setup := func(t *testing.T) (*sesThrottleFixture, *SesThrottle) {
		f := newSesThrottleFixture()
		throttle := f.NewSesThrottleFailOnErr(t)
		throttle.SentLast24Hours = throttle.Max24HourSend - 1
		return f, throttle
	}

	t.Run("SucceedsWithoutPausing", func(t *testing.T) {
		f, throttle := setup(t)
		now := f.created.Add(throttle.PauseInterval + time.Nanosecond)

		err := throttle.PauseBeforeNextSend(now)

		assert.NilError(t, err)
		assert.Equal(t, time.Duration(0), f.sleepDuration)
		assert.Assert(t, testutils.TimesEqual(now, throttle.LastSend))
		assert.Equal(t, throttle.Max24HourSend, throttle.SentLast24Hours)
	})

	t.Run("SucceedsAfterPause", func(t *testing.T) {
		f, throttle := setup(t)
		throttle.LastSend = testdata.TestTimestamp
		now := throttle.LastSend.Add(throttle.PauseInterval / 2)

		err := throttle.PauseBeforeNextSend(now)

		assert.NilError(t, err)
		assert.Equal(t, throttle.PauseInterval/2, f.sleepDuration)
		expectedSend := f.created.Add(throttle.PauseInterval)
		assert.Assert(t, testutils.TimesEqual(expectedSend, throttle.LastSend))
		assert.Equal(t, throttle.Max24HourSend, throttle.SentLast24Hours)
	})

	t.Run("ErrorsIfQuotaExhausted", func(t *testing.T) {
		f, throttle := setup(t)
		now := f.created.Add(throttle.PauseInterval + time.Nanosecond)
		throttle.SentLast24Hours = throttle.Max24HourSend

		err := throttle.PauseBeforeNextSend(now)

		assert.Assert(t, testutils.ErrorIs(err, ErrExceededMax24HourSend))
		expectedErr := fmt.Sprintf(
			"%d max, %d sent", throttle.Max24HourSend, throttle.SentLast24Hours,
		)
		assert.ErrorContains(t, err, expectedErr)
		assert.Equal(t, time.Duration(0), f.sleepDuration)
		assert.Assert(t, testutils.TimesEqual(time.Time{}, throttle.LastSend))
		assert.Equal(t, throttle.Max24HourSend, throttle.SentLast24Hours)
	})
}