package testdoubles

import (
	"context"

	"github.com/mbland/elistman/ops"
)

type Suppressor struct {
	Addresses map[string]ops.RemoveReason
	Errors    map[string]error
}

func NewSuppressor() *Suppressor {
	return &Suppressor{
		Addresses: make(map[string]ops.RemoveReason, 10),
		Errors:    make(map[string]error, 10),
	}
}

func (s *Suppressor) IsSuppressed(
	ctx context.Context, address string,
) (ok bool, err error) {
	if err = s.Errors[address]; err != nil {
		return
	}
	ok = s.Addresses[address] == ops.RemoveReasonNil
	return
}

func (s *Suppressor) Suppress(
	ctx context.Context, address string, reason ops.RemoveReason) error {
	if err := s.Errors[address]; err != nil {
		return err
	}
	s.Addresses[address] = reason
	return nil
}

func (s *Suppressor) Unsuppress(ctx context.Context, address string) error {
	if err := s.Errors[address]; err != nil {
		return err
	}
	delete(s.Addresses, address)
	return nil
}
