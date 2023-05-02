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
