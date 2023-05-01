package email

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"gotest.tools/assert"
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

func TestWriter(t *testing.T) {
	setup := func() (*strings.Builder, *writer) {
		sb := &strings.Builder{}
		return sb, &writer{buf: sb}
	}

	t.Run("WriteSucceeds", func(t *testing.T) {
		sb, w := setup()
		const msg = "Hello, World!"

		n, err := w.Write([]byte(msg))

		assert.NilError(t, err)
		assert.Equal(t, msg, sb.String())
		assert.Equal(t, len(msg), n)
	})

	t.Run("WriteStopsWritingAfterError", func(t *testing.T) {
		sb, w := setup()
		errs := make([]error, 3)
		ew := &ErrWriter{buf: sb, errorOn: "bar", err: errors.New("test error")}
		w.buf = ew

		_, errs[0] = w.Write([]byte("foo"))
		_, errs[1] = w.Write([]byte("bar"))
		_, errs[2] = w.Write([]byte("baz"))

		assert.Error(t, errors.Join(errs...), "test error")
		assert.Equal(t, sb.String(), "foo")
	})

	t.Run("WriteLineSucceeds", func(t *testing.T) {
		sb, w := setup()
		const msg = "Hello, World!"

		w.WriteLine(msg)

		assert.Equal(t, sb.String(), msg+"\r\n")
	})
}
