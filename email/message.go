package email

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"strings"
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
	from       string
	subject    string
	textBody   string
	textFooter string
	htmlBody   string
	htmlFooter string
}

func NewMessageTemplate(m *Message) *MessageTemplate {
	mt := &MessageTemplate{
		from:       "From: " + m.From,
		subject:    "Subject: " + m.Subject,
		textBody:   convertToCrlf(m.TextBody),
		textFooter: convertToCrlf(m.TextFooter),
		htmlBody:   convertToCrlf(m.HtmlBody),
		htmlFooter: convertToCrlf(m.HtmlFooter),
	}

	tb := &strings.Builder{}
	hb := &strings.Builder{}

	// strings.Builder never errors, so neither will the quotedprintable writer.
	writeQuotedPrintable(tb, mt.textBody)
	mt.textBody = tb.String()
	writeQuotedPrintable(hb, mt.htmlBody)
	mt.htmlBody = hb.String()
	return mt
}

var toHeaderPrefix = []byte("To: ")

func (mt *MessageTemplate) EmitMessage(b io.Writer, sub *Subscriber) error {
	w := &writer{buf: b}

	w.WriteLine(mt.from)
	w.Write(toHeaderPrefix)
	w.WriteLine(sub.Email)
	w.WriteLine(mt.subject)

	// If unsubHeader is empty, this is a verification message. No need for the
	// unsubscribe info if the subscriber isn't yet verified.
	if len(sub.unsubHeader) != 0 {
		w.WriteLine(sub.unsubHeader)
		w.WriteLine("List-Unsubscribe-Post: List-Unsubscribe=One-Click")
	}
	w.WriteLine("MIME-Version: 1.0")

	if len(mt.htmlBody) == 0 {
		mt.emitTextOnly(w, sub)
	} else {
		mt.emitMultipart(w, sub)
	}
	return w.err
}

var charsetUtf8 = map[string]string{"charset": "utf-8"}
var textContentType = mime.FormatMediaType("text/plain", charsetUtf8)
var htmlContentType = mime.FormatMediaType("text/html", charsetUtf8)

func (mt *MessageTemplate) emitTextOnly(w *writer, sub *Subscriber) {
	w.WriteLine("Content-Type: " + textContentType)
	w.WriteLine("Content-Transfer-Encoding: quoted-printable")
	w.WriteLine("")
	w.WriteLine(mt.textBody)

	if w.err == nil {
		w.err = writeQuotedPrintable(w, sub.FillInUnsubscribeUrl(mt.textFooter))
	}
}

func (mt *MessageTemplate) emitMultipart(w *writer, sub *Subscriber) {
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
		tf := sub.FillInUnsubscribeUrl(mt.textFooter)
		w.err = emitPart(mpw, h, textContentType, mt.textBody, tf)
	}
	if w.err == nil {
		hf := sub.FillInUnsubscribeUrl(mt.htmlFooter)
		w.err = emitPart(mpw, h, htmlContentType, mt.htmlBody, hf)
	}
	if w.err == nil {
		w.err = mpw.Close()
	}
}

func emitPart(
	w *multipart.Writer,
	h textproto.MIMEHeader,
	contentType, body, footer string,
) error {
	h.Set("Content-Type", contentType)
	if pw, err := w.CreatePart(h); err != nil {
		return err
	} else if _, err = pw.Write([]byte(body)); err != nil {
		return err
	} else {
		return writeQuotedPrintable(pw, footer)
	}
}

func writeQuotedPrintable(w io.Writer, msg string) error {
	qpw := quotedprintable.NewWriter(w)
	_, err := qpw.Write([]byte(msg))
	return errors.Join(err, qpw.Close())
}

func convertToCrlf(s string) string {
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
	return string(buf[:n])
}
