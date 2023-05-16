package types

// SentinelError is type for defining constant error values.
//
// Inspired by: https://dave.cheney.net/2019/06/10/constant-time
type SentinelError string

// Error returns the string value of a SentinelError.
func (e SentinelError) Error() string {
	return string(e)
}
