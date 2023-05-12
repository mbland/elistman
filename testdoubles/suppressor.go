package testdoubles

import "context"

type Suppressor struct {
	Addresses map[string]bool
	Errors    map[string]error
}

func NewSuppressor() *Suppressor {
	return &Suppressor{
		Addresses: make(map[string]bool, 10),
		Errors:    make(map[string]error, 10),
	}
}

func (s *Suppressor) IsSuppressed(
	ctx context.Context, address string,
) (ok bool, err error) {
	if err = s.Errors[address]; err != nil {
		return
	}
	ok = s.Addresses[address]
	return
}

func (s *Suppressor) Suppress(ctx context.Context, address string) error {
	if err := s.Errors[address]; err != nil {
		return err
	}
	s.Addresses[address] = true
	return nil
}

func (s *Suppressor) Unsuppress(ctx context.Context, address string) error {
	if err := s.Errors[address]; err != nil {
		return err
	}
	delete(s.Addresses, address)
	return nil
}
