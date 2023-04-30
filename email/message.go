package email

import (
	"errors"
	"fmt"
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

type MessageTemplate Message

func ConvertToTemplate(m *Message) (mt *MessageTemplate, err error) {
	mt = &MessageTemplate{
		From:       "From: " + m.From,
		Subject:    "Subject: " + m.Subject,
		TextBody:   convertToCrlf(m.TextBody),
		TextFooter: convertToCrlf(m.TextFooter),
		HtmlBody:   convertToCrlf(m.HtmlBody),
		HtmlFooter: convertToCrlf(m.HtmlFooter),
	}

	tb := &strings.Builder{}
	hb := &strings.Builder{}
	if err = convertBodiesToQuotedPrintable(mt, tb, hb); err != nil {
		mt = nil
	}
	mt.TextBody = tb.String()
	mt.HtmlBody = hb.String()
	return
}

var charsetUtf8 = map[string]string{"charset": "utf-8"}
var textContentType = mime.FormatMediaType("text/plain", charsetUtf8)
var htmlContentType = mime.FormatMediaType("text/html", charsetUtf8)

func (mt *MessageTemplate) EmitTextOnly(w *Writer, sub *Subscriber) {
	w.WriteLine("Content-Type: " + textContentType)
	w.WriteLine("Content-Transfer-Encoding: quoted-printable")
	w.WriteLine("")
	w.WriteLine(mt.TextBody)

	if w.err == nil {
		w.err = convertToQuotedPrintable(
			w, sub.AddUnsubscribeUrl(mt.TextFooter),
		)
	}
}

func (mt *MessageTemplate) EmitMultipart(w *Writer, sub *Subscriber) {
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
		tf := sub.AddUnsubscribeUrl(mt.TextFooter)
		w.err = emitPart(mpw, h, textContentType, mt.TextBody, tf)
	}
	if w.err == nil {
		hf := sub.AddUnsubscribeUrl(mt.HtmlFooter)
		w.err = emitPart(mpw, h, htmlContentType, mt.HtmlBody, hf)
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
	} else if _, err = pw.Write([]byte(body + "\r\n")); err != nil {
		return err
	} else {
		return convertToQuotedPrintable(pw, footer)
	}
}

func convertBodiesToQuotedPrintable(
	mt *MessageTemplate, textBuf, htmlBuf io.Writer,
) (err error) {
	if err = convertToQuotedPrintable(textBuf, mt.TextBody); err != nil {
		err = fmt.Errorf("encoding text body failed: %s", err)
	} else if err = convertToQuotedPrintable(htmlBuf, mt.HtmlBody); err != nil {
		err = fmt.Errorf("encoding html body failed: %s", err)
	}
	return
}

func convertToQuotedPrintable(w io.Writer, msg string) error {
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
