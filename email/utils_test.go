package email

import "context"

type TestSuppressor struct {
	checkedEmail       string
	isSuppressedResult bool
	suppressedEmail    string
	suppressErr        error
}

func (ts *TestSuppressor) IsSuppressed(ctx context.Context, email string) bool {
	ts.checkedEmail = email
	return ts.isSuppressedResult
}

func (ts *TestSuppressor) Suppress(ctx context.Context, email string) error {
	ts.suppressedEmail = email
	return ts.suppressErr
}
