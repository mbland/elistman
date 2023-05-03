package email

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
)

type Message struct {
	From       string
	Subject    string
	TextBody   string
	TextFooter string
	HtmlBody   string
	HtmlFooter string
}

type MessageTemplate struct {
	from       []byte
	subject    []byte
	textBody   []byte
	textFooter []byte
	htmlBody   []byte
	htmlFooter []byte
}

func NewMessageTemplate(m *Message) *MessageTemplate {
	makeHeader := func(name, value string) []byte {
		b := &bytes.Buffer{}
		b.WriteString(name)
		b.WriteString(": ")
		b.WriteString(value)
		b.Write(crlf)
		return b.Bytes()
	}

	mt := &MessageTemplate{
		from:       makeHeader("From", m.From),
		subject:    makeHeader("Subject", m.Subject),
		textBody:   convertToCrlf(m.TextBody),
		textFooter: convertToCrlf(m.TextFooter),
		htmlBody:   convertToCrlf(m.HtmlBody),
		htmlFooter: convertToCrlf(m.HtmlFooter),
	}

	tb := &bytes.Buffer{}
	hb := &bytes.Buffer{}

	// strings.Builder never errors, so neither will the quotedprintable writer.
	writeQuotedPrintable(tb, mt.textBody)
	mt.textBody = tb.Bytes()
	writeQuotedPrintable(hb, mt.htmlBody)
	mt.htmlBody = hb.Bytes()
	return mt
}

var toHeaderPrefix = []byte("To: ")
var mimeVersion = []byte("MIME-Version: 1.0\r\n")

func (mt *MessageTemplate) EmitMessage(b io.Writer, sub *Subscriber) error {
	w := &writer{buf: b}

	w.Write(mt.from)
	w.Write(toHeaderPrefix)
	w.WriteLine(sub.Email)
	w.Write(mt.subject)
	sub.EmitUnsubscribeHeaders(w)
	w.Write(mimeVersion)

	if len(mt.htmlBody) == 0 {
		mt.emitTextOnly(w, sub)
	} else {
		mt.emitMultipart(w, sub)
	}

	if w.err != nil {
		w.err = fmt.Errorf("error emitting message to %s: %s", sub.Email, w.err)
	}
	return w.err
}

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
	} else {
		n = len(b)
	}
	return
}

var contentTypeHeader = []byte("Content-Type: ")
var charsetUtf8 = map[string]string{"charset": "utf-8"}
var textContentType = mime.FormatMediaType("text/plain", charsetUtf8)
var htmlContentType = mime.FormatMediaType("text/html", charsetUtf8)
var contentEncodingQuotedPrintable = []byte(
	"Content-Transfer-Encoding: quoted-printable\r\n\r\n",
)

func (mt *MessageTemplate) emitTextOnly(w *writer, sub *Subscriber) {
	w.Write(contentTypeHeader)
	w.WriteLine(textContentType)
	w.Write(contentEncodingQuotedPrintable)
	w.Write(mt.textBody)
	err := writeQuotedPrintable(w, sub.FillInUnsubscribeUrl(mt.textFooter))

	if w.err == nil {
		w.err = err
	}
}

func (mt *MessageTemplate) emitMultipart(w *writer, sub *Subscriber) {
	mpw := multipart.NewWriter(w)
	contentType := mime.FormatMediaType(
		"multipart/alternative",
		map[string]string{"boundary": mpw.Boundary()},
	)
	w.Write(contentTypeHeader)
	w.WriteLine(contentType)
	w.Write(crlf)

	h := textproto.MIMEHeader{}
	h.Add("Content-Transfer-Encoding", "quoted-printable")

	tb := mt.textBody
	tf := sub.FillInUnsubscribeUrl(mt.textFooter)
	hb := mt.htmlBody
	hf := sub.FillInUnsubscribeUrl(mt.htmlFooter)

	if err := emitPart(mpw, h, textContentType, tb, tf); err != nil {
		w.err = err
	} else if err = emitPart(mpw, h, htmlContentType, hb, hf); err != nil {
		w.err = err
	} else if err = mpw.Close(); err != nil {
		w.err = err
	}
}

func emitPart(
	w *multipart.Writer,
	h textproto.MIMEHeader,
	contentType string,
	body, footer []byte,
) error {
	h.Set("Content-Type", contentType)
	if pw, err := w.CreatePart(h); err != nil {
		return err
	} else if _, err = pw.Write(body); err != nil {
		return err
	} else {
		return writeQuotedPrintable(pw, footer)
	}
}

func writeQuotedPrintable(w io.Writer, msg []byte) error {
	qpw := quotedprintable.NewWriter(w)
	if _, err := qpw.Write(msg); err != nil {
		return err
	}
	return qpw.Close()
}

// Per 'man ascii': 0x0d == "\r", 0x0a == "\n"
const newline byte = 0x0a
const carriageReturn byte = 0x0d

func convertToCrlf(s string) []byte {
	// Allocate enough space for a pathological string of all newlines.
	buf := make([]byte, len(s)*2)
	n := 0
	emitCr := true

	for i := range s {
		c := s[i]

		switch c {
		case newline:
			if emitCr {
				buf[n] = carriageReturn
				n++
			}
		default:
			emitCr = c != carriageReturn
		}
		buf[n] = c
		n++
	}

	// Trim the result to avoid hanging on to extra memory.
	result := make([]byte, n)
	copy(result, buf[:n])
	return result
}
