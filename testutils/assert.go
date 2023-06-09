package testutils

import (
	"errors"
	"fmt"
	"time"

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

func ErrorIsNot(err, unexpectedErr error) cmp.Comparison {
	return func() cmp.Result {
		if !errors.Is(err, unexpectedErr) {
			return cmp.ResultSuccess
		}
		const errFmt = "did not expect \"%+v\" (%T) in error tree,\n" +
			"got: \"%+v\" (%T)"
		errMsg := fmt.Sprintf(errFmt, unexpectedErr, unexpectedErr, err, err)
		return cmp.ResultFailure(errMsg)
	}
}

func TimesEqual(expectedTime, actualTime time.Time) cmp.Comparison {
	return func() cmp.Result {
		if actualTime.Equal(expectedTime) {
			return cmp.ResultSuccess
		}
		const errFmt = "times did not match:\nexpected: %s\nactual:   %s"
		errMsg := fmt.Sprintf(errFmt, expectedTime, actualTime)
		return cmp.ResultFailure(errMsg)
	}
}
