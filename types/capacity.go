package types

import "fmt"

const ErrInvalidCapacity = SentinelError("capacity not within range [0,1]")

type Capacity struct {
	cap float64
}

func NewCapacity(cap float64) (Capacity, error) {
	if cap < 0.0 || cap > 1.0 {
		return Capacity{}, fmt.Errorf("%w: %v", ErrInvalidCapacity, cap)
	}
	return Capacity{cap}, nil
}

func (c Capacity) Value() float64 {
	return c.cap
}

func (c Capacity) Equal(other Capacity) bool {
	return c.cap == other.cap
}

func (c Capacity) String() string {
	return fmt.Sprintf("%.2f%%", c.cap*100.0)
}

func (c Capacity) MaxAvailable(totalUnits int64) int64 {
	return int64(float64(totalUnits) * c.cap)
}
