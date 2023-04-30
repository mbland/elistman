package email

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
)

type Builder struct {
	buf io.Writer
}

func (b *Builder) BuildMessage(
	fromAddr, toAddr, subject, unsubHeader, textMsg, htmlMsg string,
) error {
	w := &Writer{buf: b.buf}

	w.WriteLine("From: " + fromAddr)
	w.WriteLine("To: " + toAddr)
	w.WriteLine("Subject: " + subject)
	w.WriteLine(unsubHeader)
	w.WriteLine("List-Unsubscribe-Post: List-Unsubscribe=One-Click")
	w.WriteLine("MIME-Version: 1.0")

	if len(htmlMsg) == 0 {
		buildText(w, textMsg)
	} else {
		buildMultipart(w, textMsg, htmlMsg)
	}
	return w.err
}

var charsetUtf8 = map[string]string{"charset": "utf-8"}
var textContentType = mime.FormatMediaType("text/plain", charsetUtf8)
var htmlContentType = mime.FormatMediaType("text/html", charsetUtf8)

func buildText(w *Writer, textMsg string) {
	w.WriteLine("Content-Type: " + textContentType)
	w.WriteLine("Content-Transfer-Encoding: quoted-printable")
	w.WriteLine("")
	if w.err == nil {
		w.err = emitQuotedPrintable(w, textMsg)
	}
}

func buildMultipart(w *Writer, textMsg, htmlMsg string) {
	mpw := multipart.NewWriter(w)
	contentType := mime.FormatMediaType(
		"multipart/alternative",
		map[string]string{"boundary": mpw.Boundary()},
	)
	w.WriteLine("Content-Type: " + contentType)
	w.WriteLine("")

	h := textproto.MIMEHeader{}
	h.Add("Content-Transfer-Encoding", "quoted-printable")

	if w.err == nil {
		w.err = emitPart(mpw, h, textContentType, textMsg)
	}
	if w.err == nil {
		w.err = emitPart(mpw, h, htmlContentType, htmlMsg)
	}
	if w.err == nil {
		w.err = mpw.Close()
	}
}

func emitPart(
	w *multipart.Writer, h textproto.MIMEHeader, contentType, msg string,
) error {
	h.Set("Content-Type", contentType)
	pw, err := w.CreatePart(h)

	if err != nil {
		return err
	}
	return emitQuotedPrintable(pw, msg)
}

func emitQuotedPrintable(w io.Writer, msg string) error {
	qpw := quotedprintable.NewWriter(w)
	_, err := qpw.Write([]byte(msg))
	return errors.Join(err, qpw.Close())
}

func convertToCrlf(s string) []byte {
	// Per 'man ascii':
	// - 0x0d == "\r"
	// - 0x0a == "\n"
	numLf := 0
	for i := range s {
		if s[i] == 0x0a {
			numLf++
		}
	}

	buf := make([]byte, len(s)+numLf)
	n := 0
	emitCr := true

	for i := range s {
		c := s[i]
		switch c {
		case 0x0a:
			if emitCr {
				buf[n] = 0x0d
				n++
			}
		default:
			emitCr = c != 0x0d
		}
		buf[n] = c
		n++
	}
	return buf[:n]
}
