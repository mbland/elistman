//go:build small_tests || all_tests

package email

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	sleepDuration time.Duration
	sleep         func(time.Duration)
	now           time.Time
	refresh       time.Duration
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
		now:      testdata.TestTimestamp,
		refresh:  time.Minute,
	}
	f.client.getAccountOutput = &sesv2.GetAccountOutput{SendQuota: f.quota}
	f.sleep = func(sleepFor time.Duration) {
		f.sleepDuration = sleepFor
	}
	return f
}

func (f *sesThrottleFixture) NewSesThrottle() (*SesThrottle, error) {
	now := func() time.Time { return f.now }
	return NewSesThrottle(f.ctx, f.client, f.capacity, f.sleep, now, f.refresh)
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
		assert.Equal(t, f.now, throttle.Updated)
		assert.Equal(t, time.Duration(time.Second/25), throttle.PauseInterval)
		assert.Assert(t, testutils.TimesEqual(time.Time{}, throttle.LastSend))
		assert.Equal(t, time.Second, f.sleepDuration)
		assert.Equal(t, f.refresh, throttle.RefreshInterval)
		assert.Equal(t, int64(f.quota.Max24HourSend), throttle.Max24HourSend)
		assert.Assert(t, throttle.unlimited() == false)
		assert.Equal(
			t, int64(f.quota.SentLast24Hours), throttle.SentLast24Hours,
		)
		assert.Equal(t, f.capacity.Value(), throttle.MaxBulkCapacity.Value())
		assert.Equal(t, int64(37500), throttle.MaxBulkSendable)
	})

	t.Run("SucceedsWithUnlimitedQuota", func(t *testing.T) {
		f := newSesThrottleFixture()
		f.quota.Max24HourSend = float64(-1)

		throttle, err := f.NewSesThrottle()
		assert.NilError(t, err)
		assert.Assert(t, throttle.unlimited() == true)
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
		f.now = throttle.Updated.Add(f.refresh - time.Nanosecond)

		err := throttle.refresh(f.ctx)

		assert.NilError(t, err)
		assert.Equal(t, int64(f.quota.Max24HourSend), throttle.Max24HourSend)
	})

	t.Run("RefreshesIfExpired", func(t *testing.T) {
		f, throttle := setup(t)
		f.now = throttle.Updated.Add(f.refresh)

		err := throttle.refresh(f.ctx)

		assert.NilError(t, err)
		assert.Equal(t, int64(f.quota.Max24HourSend)*2, throttle.Max24HourSend)
	})
}

func TestBulkCapacityAvailable(t *testing.T) {
	setup := func(t *testing.T) (*sesThrottleFixture, *SesThrottle) {
		f := newSesThrottleFixture()
		return f, f.NewSesThrottleFailOnErr(t)
	}

	t.Run("Succeeds", func(t *testing.T) {
		f, throttle := setup(t)
		numToSend := throttle.MaxBulkSendable - throttle.SentLast24Hours

		err := throttle.BulkCapacityAvailable(f.ctx, numToSend)

		assert.NilError(t, err)
	})

	t.Run("AlwaysSucceedsIfUnlimited", func(t *testing.T) {
		f, throttle := setup(t)
		throttle.Max24HourSend = -1

		err := throttle.BulkCapacityAvailable(f.ctx, math.MaxInt)

		assert.NilError(t, err)
	})

	t.Run("ErrorsIfRefreshFails", func(t *testing.T) {
		f, throttle := setup(t)
		numToSend := throttle.MaxBulkSendable - throttle.SentLast24Hours
		f.now = throttle.Updated.Add(f.refresh)
		f.client.getAccountError = errors.New("test error")

		err := throttle.BulkCapacityAvailable(f.ctx, numToSend)

		assert.Error(t, err, "failed to get AWS account info: test error")
	})

	t.Run("ErrorsIfInsufficientCapacity", func(t *testing.T) {
		f, throttle := setup(t)
		numToSend := throttle.MaxBulkSendable - throttle.SentLast24Hours + 1

		err := throttle.BulkCapacityAvailable(f.ctx, numToSend)

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
		f.now = f.now.Add(throttle.PauseInterval + time.Nanosecond)

		err := throttle.PauseBeforeNextSend(f.ctx)

		assert.NilError(t, err)
		assert.Equal(t, time.Duration(0), f.sleepDuration)
		assert.Assert(t, testutils.TimesEqual(f.now, throttle.LastSend))
		assert.Equal(t, throttle.Max24HourSend, throttle.SentLast24Hours)
	})

	t.Run("SucceedsAfterPause", func(t *testing.T) {
		f, throttle := setup(t)
		throttle.LastSend = testdata.TestTimestamp
		origNow := f.now
		f.now = throttle.LastSend.Add(throttle.PauseInterval / 2)

		err := throttle.PauseBeforeNextSend(f.ctx)

		assert.NilError(t, err)
		assert.Equal(t, throttle.PauseInterval/2, f.sleepDuration)
		expectedSend := origNow.Add(throttle.PauseInterval)
		assert.Assert(t, testutils.TimesEqual(expectedSend, throttle.LastSend))
		assert.Equal(t, throttle.Max24HourSend, throttle.SentLast24Hours)
	})

	t.Run("SucceedsWithUnlimitedQuota", func(t *testing.T) {
		f, throttle := setup(t)
		throttle.Max24HourSend = -1
		throttle.SentLast24Hours = math.MaxInt64 - 1

		err := throttle.PauseBeforeNextSend(f.ctx)

		assert.NilError(t, err)
		assert.Equal(t, int64(math.MaxInt64), throttle.SentLast24Hours)
	})

	t.Run("ErrorsIfRefreshFails", func(t *testing.T) {
		f, throttle := setup(t)
		f.now = throttle.Updated.Add(f.refresh)
		f.client.getAccountError = errors.New("test error")

		err := throttle.PauseBeforeNextSend(f.ctx)

		assert.Error(t, err, "failed to get AWS account info: test error")
	})

	t.Run("ErrorsIfQuotaExhausted", func(t *testing.T) {
		f, throttle := setup(t)
		f.now = f.now.Add(throttle.PauseInterval + time.Nanosecond)
		throttle.SentLast24Hours = throttle.Max24HourSend

		err := throttle.PauseBeforeNextSend(f.ctx)

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
