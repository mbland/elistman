package ops

import "github.com/mbland/elistman/types"

// ErrExternal indicates that a request to an upstream service failed.
//
// handler.Handler checks for this error in order to return an HTTP 502 when
// applicable.
const ErrExternal = types.SentinelError("external error")
