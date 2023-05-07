//go:build small_tests || medium_tests || all_tests

package email

import (
	"bytes"
	"context"
	"io"
)

type ErrWriter struct {
	buf     io.Writer
	errorOn string
	err     error
}

func (ew *ErrWriter) Write(b []byte) (int, error) {
	if bytes.Contains(b, []byte(ew.errorOn)) {
		return 0, ew.err
	}
	return ew.buf.Write(b)
}

const testUnsubEmail = "unsubscribe@foo.com"
const testUnsubBaseUrl = "https://foo.com/email/unsubscribe/"
const testUid = "00000000-1111-2222-3333-444444444444"

type TestSuppressor struct {
	checkedEmail       string
	isSuppressedResult bool
	suppressedEmail    string
	suppressErr        error
	unsuppressedEmail  string
	unsuppressErr      error
}

func (ts *TestSuppressor) IsSuppressed(ctx context.Context, email string) bool {
	ts.checkedEmail = email
	return ts.isSuppressedResult
}

func (ts *TestSuppressor) Suppress(ctx context.Context, email string) error {
	ts.suppressedEmail = email
	return ts.suppressErr
}

func (ts *TestSuppressor) Unsuppress(ctx context.Context, email string) error {
	ts.unsuppressedEmail = email
	return ts.unsuppressErr
}
