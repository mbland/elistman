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
	BulkCapacityAvailable(ctx context.Context, numToSend int) error
	PauseBeforeNextSend(context.Context) error
}

type SesThrottle struct {
	Client          SesV2Api
	Updated         time.Time
	PauseInterval   time.Duration
	LastSend        time.Time
	Sleep           func(time.Duration)
	Now             func() time.Time
	RefreshInterval time.Duration
	Max24HourSend   int
	SentLast24Hours int
	MaxBulkCapacity types.Capacity
	MaxBulkSendable int
}

func NewSesThrottle(
	ctx context.Context,
	client SesV2Api,
	maxCap types.Capacity,
	sleep func(time.Duration),
	now func() time.Time,
	refreshInterval time.Duration,
) (t *SesThrottle, err error) {
	throttle := &SesThrottle{
		Client:          client,
		Sleep:           sleep,
		Now:             now,
		RefreshInterval: refreshInterval,
		MaxBulkCapacity: maxCap,
	}
	if err = throttle.refresh(ctx); err == nil {
		t = throttle
	}
	return
}

func (t *SesThrottle) BulkCapacityAvailable(
	ctx context.Context, numToSend int,
) (err error) {
	if err = t.refresh(ctx); err != nil {
		return
	} else if (t.MaxBulkSendable - t.SentLast24Hours) < numToSend {
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

func (t *SesThrottle) PauseBeforeNextSend(ctx context.Context) (err error) {
	if err = t.refresh(ctx); err != nil {
		return
	} else if t.SentLast24Hours >= t.Max24HourSend {
		err = fmt.Errorf(
			"%w: %d max, %d sent",
			ErrExceededMax24HourSend,
			t.Max24HourSend,
			t.SentLast24Hours,
		)
		return
	}
	t.LastSend = t.LastSend.Add(t.PauseInterval)
	now := t.Now()

	if t.LastSend.Before(now) {
		t.LastSend = now
	} else {
		t.Sleep(t.LastSend.Sub(now))
	}
	t.SentLast24Hours++
	return
}

func (t *SesThrottle) refresh(ctx context.Context) (err error) {
	now := t.Now()

	if now.Sub(t.Updated) < t.RefreshInterval {
		return nil
	}

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
	t.Updated = now
	return
}
