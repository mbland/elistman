package testutils

import (
	"errors"
	"fmt"

	"gotest.tools/assert/cmp"
)

func ErrorIs(err, expectedErr error) cmp.Comparison {
	return func() cmp.Result {
		if errors.Is(err, expectedErr) {
			return cmp.ResultSuccess
		}
		const errFmt = "expected \"%+v\" (%T) in error tree,\ngot: \"%+v\" (%T)"
		errMsg := fmt.Sprintf(errFmt, expectedErr, expectedErr, err, err)
		return cmp.ResultFailure(errMsg)
	}
}
