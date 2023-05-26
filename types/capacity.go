package types

import "fmt"

type Capacity struct {
	cap float64
}

func NewCapacity(cap float64) Capacity {
	if cap < 0.0 || cap > 1.0 {
		panic(fmt.Sprintf("capacity must be within range [0,1], got: %v", cap))
	}
	return Capacity{cap}
}

func (c Capacity) Value() float64 {
	return c.cap
}

func (c Capacity) String() string {
	return fmt.Sprintf("%.2f%%", c.cap*100.0)
}

func (c Capacity) MaxAvailable(totalUnits int) int {
	return int(float64(totalUnits) * c.cap)
}
