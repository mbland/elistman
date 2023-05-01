package email

import (
	"io"
)

type writer struct {
	buf io.Writer
	err error
}

var crlf = []byte("\r\n")

func (w *writer) WriteLine(s string) {
	w.Write([]byte(s))
	w.Write(crlf)
}

func (w *writer) Write(b []byte) (n int, err error) {
	if w.err == nil {
		n, err = w.buf.Write(b)
		w.err = err
	}
	return
}
