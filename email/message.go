package email

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
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

// MessageValidatorFunc is the interface for Message.Validate validators.
//
// These functions are applied after all other Message.Validate checks,
// including the parsing of the From address. Validate passes the result of a
// successful parse via the fromName and fromAddress parameters.
type MessageValidatorFunc func(msg *Message, fromName, fromAddress string) error

func (msg *Message) Validate(validators ...MessageValidatorFunc) error {
	errs := make([]error, 0, 5)
	addErr := func(msg string) {
		errs = append(errs, errors.New(msg))
	}

	var fromName string
	var fromAddress string

	if len(msg.From) == 0 {
		addErr("missing From")
	} else if addr, err := mail.ParseAddress(msg.From); err != nil {
		addErr("failed to parse From address \"" + msg.From + "\": " +
			err.Error())
	} else {
		fromName = addr.Name
		fromAddress = addr.Address
	}
	if len(msg.Subject) == 0 {
		addErr("missing Subject")
	}
	if len(msg.TextBody) == 0 {
		addErr("missing TextBody")
	}
	if len(msg.TextFooter) == 0 {
		addErr("missing TextFooter")
	} else if !strings.Contains(msg.TextFooter, UnsubscribeUrlTemplate) {
		addErr("TextFooter does not contain " + UnsubscribeUrlTemplate)
	}
	if len(msg.HtmlBody) != 0 {
		if len(msg.HtmlFooter) == 0 {
			addErr("HtmlFooter missing")
		} else if !strings.Contains(msg.HtmlFooter, UnsubscribeUrlTemplate) {
			addErr("HtmlFooter does not contain " + UnsubscribeUrlTemplate)
		}
	} else if len(msg.HtmlFooter) != 0 {
		addErr("HtmlFooter present, but HtmlBody missing")
	}

	for _, vf := range validators {
		errs = append(errs, vf(msg, fromName, fromAddress))
	}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("message failed validation: %w", err)
	}
	return nil
}

// CheckDomain ensures Message.From is from the expected domain.
func CheckDomain(domain string) MessageValidatorFunc {
	return func(_ *Message, _, addr string) (err error) {
		// If the address failed to parse, addr will be empty. No need to add an
		// extra error message, as Message.Validate will have emitted one for
		// the parse failure.
		if addr != "" && strings.Split(addr, "@")[1] != domain {
			return errors.New("domain of From address is not " + domain)
		}
		return
	}
}

type MessageTemplate struct {
	from       []byte
	subject    []byte
	textBody   []byte
	textFooter []byte
	htmlBody   []byte
	htmlFooter []byte
}

func NewMessageTemplateFromJson(
	r io.Reader, validators ...MessageValidatorFunc,
) (mt *MessageTemplate, err error) {
	var msgJson []byte
	msg := &Message{}
	if msgJson, err = io.ReadAll(r); err != nil {
		err = fmt.Errorf("failed to read JSON from input: %w", err)
	} else if err = json.Unmarshal(msgJson, msg); err != nil {
		err = fmt.Errorf("failed to parse message input from JSON: %w", err)
	} else if err = msg.Validate(validators...); err == nil {
		mt = NewMessageTemplate(msg)
	}
	return
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
		textBody:   convertToCrlf(appendNewlineIfNeeded(m.TextBody)),
		textFooter: convertToCrlf(m.TextFooter),
		htmlBody:   convertToCrlf(appendNewlineIfNeeded(m.HtmlBody)),
		htmlFooter: convertToCrlf(m.HtmlFooter),
	}

	tb := &bytes.Buffer{}
	hb := &bytes.Buffer{}

	// bytest.Buffer never errors, so neither will the quotedprintable writer.
	writeQuotedPrintable(tb, mt.textBody)
	mt.textBody = tb.Bytes()
	writeQuotedPrintable(hb, mt.htmlBody)
	mt.htmlBody = hb.Bytes()
	return mt
}

var toHeaderPrefix = []byte("To: ")
var mimeVersion = []byte("MIME-Version: 1.0\r\n")

func appendNewlineIfNeeded(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func (mt *MessageTemplate) GenerateMessage(r *Recipient) []byte {
	// Don't check the EmitMessage error because bytes.Buffer can essentially
	// never return an error. If it runs out of memory, it panics.
	buf := &bytes.Buffer{}
	mt.EmitMessage(buf, r)
	return buf.Bytes()
}

func (mt *MessageTemplate) EmitMessage(b io.Writer, r *Recipient) error {
	w := &writer{buf: b}

	w.Write(mt.from)
	w.Write(toHeaderPrefix)
	w.WriteLine(r.Email)
	w.Write(mt.subject)
	r.EmitUnsubscribeHeaders(w)
	w.Write(mimeVersion)

	if len(mt.htmlBody) == 0 {
		mt.emitTextOnly(w, r)
	} else {
		mt.emitMultipart(w, r)
	}

	if w.err != nil {
		w.err = fmt.Errorf("error emitting message to %s: %s", r.Email, w.err)
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

func (mt *MessageTemplate) emitTextOnly(w *writer, sub *Recipient) {
	w.Write(contentTypeHeader)
	w.WriteLine(textContentType)
	w.Write(contentEncodingQuotedPrintable)
	w.Write(mt.textBody)
	err := writeQuotedPrintable(w, sub.FillInUnsubscribeUrl(mt.textFooter))

	if w.err == nil {
		w.err = err
	}
}

func (mt *MessageTemplate) emitMultipart(w *writer, sub *Recipient) {
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
