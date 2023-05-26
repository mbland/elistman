package email

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/mbland/elistman/ops"
	"github.com/mbland/elistman/types"
)

const ErrExceededMax24HourSend = types.SentinelError(
	"Exceeded 24 hour maximum send quota",
)

const ErrBulkSendWouldExceedCapacity = types.SentinelError(
	"Sending items would exceed bulk capacity for 24 hour max send quota",
)

type Throttle interface {
	RefreshIfExpired(
		ctx context.Context, maxAge time.Duration, now time.Time,
	) error
	BulkCapacityAvailable(numToSend int) error
	PauseBeforeNextSend(now time.Time) error
}

type SesThrottle struct {
	Client          SesV2Api
	Created         time.Time
	PauseInterval   time.Duration
	LastSend        time.Time
	Sleep           func(time.Duration)
	Max24HourSend   int
	SentLast24Hours int
	MaxBulkCapacity types.Capacity
	MaxBulkSendable int
}

func NewSesThrottle(
	ctx context.Context,
	client SesV2Api,
	maxCap types.Capacity,
	now time.Time,
	sleep func(time.Duration),
) (t *SesThrottle, err error) {
	throttle := &SesThrottle{
		Client:          client,
		Created:         now,
		Sleep:           sleep,
		MaxBulkCapacity: maxCap,
	}
	if err = throttle.Refresh(ctx); err == nil {
		t = throttle
	}
	return
}

func (t *SesThrottle) RefreshIfExpired(
	ctx context.Context, maxAge time.Duration, now time.Time,
) error {
	if now.Sub(t.Created) < maxAge {
		return nil
	}
	return t.Refresh(ctx)
}

func (t *SesThrottle) Refresh(ctx context.Context) (err error) {
	input := &sesv2.GetAccountInput{}
	var output *sesv2.GetAccountOutput

	if output, err = t.Client.GetAccount(ctx, input); err != nil {
		return ops.AwsError("failed to get AWS account info", err)
	}
	quota := output.SendQuota

	t.PauseInterval = time.Duration(float64(time.Second) / quota.MaxSendRate)
	t.Max24HourSend = int(quota.Max24HourSend)
	t.SentLast24Hours = int(quota.SentLast24Hours)
	t.MaxBulkSendable = t.MaxBulkCapacity.MaxAvailable(t.Max24HourSend)
	return
}

func (t *SesThrottle) BulkCapacityAvailable(numToSend int) (err error) {
	if (t.MaxBulkSendable - t.SentLast24Hours) < numToSend {
		const errFmt = "%w: %d total send max, %s desired bulk capacity, " +
			"%d bulk sendable, %d sent last 24h, %d requested"
		err = fmt.Errorf(
			errFmt,
			ErrBulkSendWouldExceedCapacity,
			t.Max24HourSend,
			t.MaxBulkCapacity,
			t.MaxBulkSendable,
			t.SentLast24Hours,
			numToSend,
		)
	}
	return
}

func (t *SesThrottle) PauseBeforeNextSend(now time.Time) (err error) {
	if t.SentLast24Hours >= t.Max24HourSend {
		err = fmt.Errorf(
			"%w: %d max, %d sent",
			ErrExceededMax24HourSend,
			t.Max24HourSend,
			t.SentLast24Hours,
		)
		return
	}
	t.LastSend = t.LastSend.Add(t.PauseInterval)

	if t.LastSend.Before(now) {
		t.LastSend = now
	} else {
		t.Sleep(t.LastSend.Sub(now))
	}
	t.SentLast24Hours++
	return
}
