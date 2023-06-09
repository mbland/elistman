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

const ErrBulkSendCapacityExhausted = types.SentinelError(
	"Bulk capacity for 24 hour max send quota already consumed",
)

type Throttle interface {
	BulkCapacityAvailable(ctx context.Context) error
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
	Max24HourSend   int64
	SentLast24Hours int64
	MaxBulkCapacity types.Capacity
	MaxBulkSendable int64
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

func (t *SesThrottle) BulkCapacityAvailable(ctx context.Context) (err error) {
	if err = t.refresh(ctx); err != nil || t.unlimited() {
		return
	} else if t.MaxBulkSendable < t.SentLast24Hours {
		const errFmt = "%w: %d total send max, %s designated bulk capacity, " +
			"%d bulk sendable, %d sent last 24h"
		err = fmt.Errorf(
			errFmt,
			ErrBulkSendCapacityExhausted,
			t.Max24HourSend,
			t.MaxBulkCapacity,
			t.MaxBulkSendable,
			t.SentLast24Hours,
		)
	}
	return
}

func (t *SesThrottle) PauseBeforeNextSend(ctx context.Context) (err error) {
	if err = t.refresh(ctx); err != nil {
		return
	} else if !t.unlimited() && t.SentLast24Hours >= t.Max24HourSend {
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

func (t *SesThrottle) unlimited() bool {
	// https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_SendQuota.html
	//
	// Max24HourSend
	//
	// The maximum number of emails that you can send in the current AWS
	// Region over a 24-hour period. A value of -1 signifies an unlimited
	// quota. (This value is also referred to as your sending quota.)
	return t.Max24HourSend == -1
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
	t.Max24HourSend = int64(quota.Max24HourSend)
	t.SentLast24Hours = int64(quota.SentLast24Hours)
	t.MaxBulkSendable = t.MaxBulkCapacity.MaxAvailable(t.Max24HourSend)
	t.Updated = now
	return
}
