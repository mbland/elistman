package email

import (
	"io"
)

type Writer struct {
	buf io.Writer
	err error
}

func (w *Writer) WriteLine(s string) {
	if w.err == nil {
		_, w.err = w.buf.Write([]byte(s + "\r\n"))
	}
}

func (w *Writer) Write(b []byte) (n int, err error) {
	if w.err == nil {
		n, err = w.buf.Write(b)
		w.err = err
	}
	return
}
