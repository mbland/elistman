//go:build small_tests || medium_tests || all_tests

package email

import (
	"context"
)

const testUnsubEmail = "unsubscribe@foo.com"
const testApiBaseUrl = "https://foo.com/email"
const testUid = "00000000-1111-2222-3333-444444444444"

type TestSuppressor struct {
	checkedEmail       string
	isSuppressedResult bool
	isSuppressedErr    error
	suppressedEmail    string
	suppressErr        error
	unsuppressedEmail  string
	unsuppressErr      error
}

func (ts *TestSuppressor) IsSuppressed(
	ctx context.Context, email string,
) (bool, error) {
	ts.checkedEmail = email
	return ts.isSuppressedResult, ts.isSuppressedErr
}

func (ts *TestSuppressor) Suppress(ctx context.Context, email string) error {
	ts.suppressedEmail = email
	return ts.suppressErr
}

func (ts *TestSuppressor) Unsuppress(ctx context.Context, email string) error {
	ts.unsuppressedEmail = email
	return ts.unsuppressErr
}
