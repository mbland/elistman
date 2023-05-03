//go:build small_tests || all_tests

package email

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

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

func TestConvertToCrlf(t *testing.T) {
	checkCrlfOutput := func(t *testing.T, before, expected string) {
		t.Helper()
		actual := string(convertToCrlf(before))
		assert.Check(t, is.Equal(expected, actual))
	}

	t.Run("LeavesStringsWithoutNewlinesUnchanged", func(t *testing.T) {
		checkCrlfOutput(t, "", "")
		checkCrlfOutput(t, "\r", "\r")
	})

	t.Run("LeavesStringsAlreadyContainingCrlfUnchanged", func(t *testing.T) {
		checkCrlfOutput(t, "foo\r\nbar\r\nbaz", "foo\r\nbar\r\nbaz")
	})

	t.Run("AddsCarriageFeedBeforeNewlineAsNeeded", func(t *testing.T) {
		checkCrlfOutput(t, "\n", "\r\n")
		checkCrlfOutput(t, "foo\nbar\nbaz\n", "foo\r\nbar\r\nbaz\r\n")
		checkCrlfOutput(t, "foo\r\nbar\nbaz", "foo\r\nbar\r\nbaz")
	})

	t.Run("DoesNotAddNewlinesAfterCarriageFeeds", func(t *testing.T) {
		checkCrlfOutput(t, "foo\rbar\nbaz", "foo\rbar\r\nbaz")
	})

	t.Run("TrimsResultToExactCapacity", func(t *testing.T) {
		result := convertToCrlf("foo\nbar\nbaz")

		assert.Equal(t, cap(result), len(result))
	})
}

func TestWriteQuotedPrintable(t *testing.T) {
	setup := func() (*strings.Builder, *ErrWriter) {
		sb := &strings.Builder{}
		return sb, &ErrWriter{buf: sb}
	}

	t.Run("Succeeds", func(t *testing.T) {
		sb, _ := setup()
		msg := "This message is longer than 76 chars so we can see " +
			"the quoted-printable encoding kick in.\r\n" +
			"\r\n" +
			"It also contains <a href=\"https://foo.com/\">a hyperlink</a>, " +
			"in which the equals sign will be encoded."

		err := writeQuotedPrintable(sb, []byte(msg))

		assert.NilError(t, err)
		expected := "This message is longer than 76 chars so we can see " +
			"the quoted-printable enc=\r\n" +
			"oding kick in.\r\n" +
			"\r\n" +
			`It also contains <a href=3D"https://foo.com/">a hyperlink</a>, ` +
			"in which the=\r\n" +
			" equals sign will be encoded."
		actual := sb.String()
		assert.Equal(t, expected, actual)
	})

	t.Run("ReturnsWriteError", func(t *testing.T) {
		_, ew := setup()
		msg := "This message will trigger an artificial Write error " +
			"when the first 76 characters are flushed."
		ew.errorOn = "trigger an artificial Write error"
		ew.err = errors.New("Write error")

		assert.Error(t, writeQuotedPrintable(ew, []byte(msg)), "Write error")
	})

	t.Run("ReturnsCloseError", func(t *testing.T) {
		_, ew := setup()
		msg := "Close will fail when it calls flush on this short message."
		ew.errorOn = "Close will fail"
		ew.err = errors.New("Close error")

		assert.Error(t, writeQuotedPrintable(ew, []byte(msg)), "Close error")
	})
}

var testMessage *Message = &Message{
	From:    "EListMan@foo.com",
	Subject: "This is a test",

	TextBody: "This is only a test.\n" +
		"\n" +
		"This message body is over 76 characters wide " +
		"so we can see quoted-printable encoding in the MessageTemplate.\n",
	TextFooter: "\nUnsubscribe: " + UnsubscribeUrlTemplate + "\n" +
		"This footer is over 76 characters wide, " +
		"but will be quoted-printable encoded by EmitMessage.",

	HtmlBody: "<!DOCTYPE html>\n" +
		"<html><head><title>This is a test</title></head>\n" +
		"<body><p>This is only a test.</p>\n" +
		"\n" +
		"<p>This message body is over 76 characters wide " +
		"so we can see quoted-printable encoding in the MessageTemplate.</p>\n",
	HtmlFooter: "\n<p><a href=\"" + UnsubscribeUrlTemplate +
		"\">Unsubscribe</a></p>\n" +
		"<p>This footer is over 76 characters wide, " +
		"but will be quoted-printable encoded by EmitMessage.</p>\n" +
		"</body></html>",
}

var testTemplate *MessageTemplate = &MessageTemplate{
	from:    []byte("From: EListMan@foo.com\r\n"),
	subject: []byte("Subject: This is a test\r\n"),

	textBody: []byte("This is only a test.\r\n" +
		"\r\n" +
		"This message body is over 76 characters wide " +
		"so we can see quoted-printable=\r\n" +
		" encoding in the MessageTemplate.\r\n"),
	textFooter: []byte("\r\n" +
		"Unsubscribe: " + UnsubscribeUrlTemplate + "\r\n" +
		"This footer is over 76 characters wide, " +
		"but will be quoted-printable encoded by EmitMessage."),

	htmlBody: []byte("<!DOCTYPE html>\r\n" +
		"<html><head><title>This is a test</title></head>\r\n" +
		"<body><p>This is only a test.</p>\r\n" +
		"\r\n" +
		"<p>This message body is over 76 characters wide " +
		"so we can see quoted-printa=\r\n" +
		"ble encoding in the MessageTemplate.</p>\r\n"),
	htmlFooter: []byte("\r\n" +
		"<p><a href=\"" + UnsubscribeUrlTemplate +
		"\">Unsubscribe</a></p>\r\n" +
		"<p>This footer is over 76 characters wide, " +
		"but will be quoted-printable encoded by EmitMessage.</p>\r\n" +
		"</body></html>"),
}

func byteStringsEqual(t *testing.T, expected, actual []byte) {
	t.Helper()
	assert.Check(t, is.Equal(string(expected), string(actual)))
}

func TestNewMessageTemplate(t *testing.T) {
	mt := NewMessageTemplate(testMessage)

	byteStringsEqual(t, testTemplate.from, mt.from)
	byteStringsEqual(t, testTemplate.subject, mt.subject)
	byteStringsEqual(t, testTemplate.textBody, mt.textBody)
	byteStringsEqual(t, testTemplate.textFooter, mt.textFooter)
	byteStringsEqual(t, testTemplate.htmlBody, mt.htmlBody)
	byteStringsEqual(t, testTemplate.htmlFooter, mt.htmlFooter)
}

var testSubscriber *Subscriber = &Subscriber{
	Email: "subscriber@foo.com",
	Uid:   uuid.MustParse(testUid),
}

func newTestSubscriber() *Subscriber {
	var sub Subscriber = *testSubscriber
	return &sub
}

var instantiatedTextFooter = []byte("\r\n" +
	"Unsubscribe: https://foo.com/email/unsubscribe/" +
	"subscriber@foo.com/00000000-1111-2222-3333-444444444444\r\n" +
	"This footer is over 76 characters wide, " +
	"but will be quoted-printable encoded by EmitMessage.")

var encodedTextFooter = []byte("\r\n" +
	"Unsubscribe: https://foo.com/email/unsubscribe/" +
	"subscriber@foo.com/00000000-=\r\n" +
	"1111-2222-3333-444444444444\r\n" +
	"This footer is over 76 characters wide, " +
	"but will be quoted-printable encode=\r\n" +
	"d by EmitMessage.")

var textOnlyContent = "Content-Type: " + textContentType + "\r\n" +
	string(contentEncodingQuotedPrintable) +
	string(testTemplate.textBody) +
	string(encodedTextFooter)

var decodedTextContent = string(convertToCrlf(testMessage.TextBody)) +
	string(instantiatedTextFooter)

func parseMessage(t *testing.T, content string) (msg *mail.Message) {
	t.Helper()
	var err error

	if msg, err = mail.ReadMessage(strings.NewReader(content)); err != nil {
		t.Fatalf("couldn't parse message from content: %s\n%s", err, content)
	}
	return
}

func assertValue(t *testing.T, name, expected, actual string) {
	t.Helper()

	if expected != actual {
		t.Fatalf("expected %s: %s, actual: %s", name, expected, actual)
	}
}

func assertContentTypeAndGetParams(
	t *testing.T, headers textproto.MIMEHeader, expectedMediaType string,
) (params map[string]string) {
	t.Helper()

	var mediaType string
	var err error

	if ct := headers.Get("Content-Type"); ct == "" {
		t.Fatalf("no Content-Type header in: %+v", headers)
	} else if mediaType, params, err = mime.ParseMediaType(ct); err != nil {
		t.Fatalf("couldn't parse media type from: %s: %s", ct, err)
	} else {
		assertValue(t, "media type", expectedMediaType, mediaType)
	}
	return
}

func assertContentType(
	t *testing.T,
	headers textproto.MIMEHeader,
	expectedMediaType string,
	expectedParams map[string]string,
) {
	t.Helper()

	params := assertContentTypeAndGetParams(t, headers, expectedMediaType)
	const assertMsg = "unexpected Content-Type params"
	assert.Assert(t, is.DeepEqual(expectedParams, params), assertMsg)
}

func assertDecodedContent(t *testing.T, content io.Reader, expected string) {
	t.Helper()

	if decoded, err := io.ReadAll(content); err != nil {
		t.Errorf("couldn't read and decode content: %s", err)
	} else {
		actual := string(decoded)
		assert.Equal(t, expected, actual)
	}
}

func parseTextMessage(
	t *testing.T, content string,
) (msg *mail.Message, qpReader *quotedprintable.Reader) {
	t.Helper()

	msg = parseMessage(t, content)
	header := textproto.MIMEHeader(msg.Header)
	assertContentType(t, header, "text/plain", charsetUtf8)

	const cte = "Content-Transfer-Encoding"
	assertValue(t, cte, "quoted-printable", header.Get(cte))

	qpReader = quotedprintable.NewReader(msg.Body)
	return
}

func TestEmitTextOnly(t *testing.T) {
	setup := func() (*strings.Builder, *writer, *ErrWriter, *Subscriber) {
		sb := &strings.Builder{}
		sub := newTestSubscriber()
		sub.SetUnsubscribeInfo(testUnsubEmail, testUnsubBaseUrl)
		return sb, &writer{buf: sb}, &ErrWriter{buf: sb}, sub
	}

	t.Run("Succeeds", func(t *testing.T) {
		sb, w, _, sub := setup()

		testTemplate.emitTextOnly(w, sub)

		assert.NilError(t, w.err)
		assert.Equal(t, textOnlyContent, sb.String())
		_, qpr := parseTextMessage(t, sb.String())
		assertDecodedContent(t, qpr, decodedTextContent)
	})

	t.Run("ReturnsWriteQuotedPrintableError", func(t *testing.T) {
		_, w, ew, sub := setup()
		w.buf = ew
		ew.errorOn = "Unsubscribe: "
		ew.err = errors.New("writeQuotedPrintable error")

		testTemplate.emitTextOnly(w, sub)

		assert.Error(t, w.err, "writeQuotedPrintable error")
	})
}

var textPart string = "Content-Transfer-Encoding: quoted-printable\r\n" +
	"Content-Type: " + textContentType + "\r\n" +
	"\r\n" +
	string(testTemplate.textBody) +
	string(encodedTextFooter)

func newPartReader(content io.Reader, boundary string) *multipart.Reader {
	return multipart.NewReader(content, boundary)
}

func assertNextPart(
	t *testing.T, reader *multipart.Reader, mediaType, decoded string,
) {
	t.Helper()

	var part *multipart.Part
	var err error

	if part, err = reader.NextPart(); err != nil {
		t.Fatalf("couldn't parse message part: %s", err)
	}
	assertContentType(t, part.Header, mediaType, charsetUtf8)

	// Per: https://pkg.go.dev/mime/multipart#Reader.NextPart
	//
	// > As a special case, if the "Content-Transfer-Encoding" header has a
	// > value of "quoted-printable", that header is instead hidden and the body
	// > is transparently decoded during Read calls.
	const cte = "Content-Transfer-Encoding"
	assertValue(t, cte, "", part.Header.Get(cte))
	assertDecodedContent(t, part, decoded)
}

func TestEmitPart(t *testing.T) {
	setup := func() (
		*strings.Builder,
		textproto.MIMEHeader,
		*multipart.Writer,
	) {
		sb := &strings.Builder{}
		w := &writer{buf: sb}
		h := textproto.MIMEHeader{}
		h.Add("Content-Transfer-Encoding", "quoted-printable")
		return sb, h, multipart.NewWriter(w)
	}

	setupErrWriter := func(errorMsg string) (
		*ErrWriter, textproto.MIMEHeader, *multipart.Writer,
	) {
		sb, h, _ := setup()
		ew := &ErrWriter{buf: sb}
		ew.err = errors.New(errorMsg)
		return ew, h, multipart.NewWriter(ew)
	}

	contentType := textContentType
	body := testTemplate.textBody
	footer := instantiatedTextFooter

	t.Run("Succeeds", func(t *testing.T) {
		sb, h, mpw := setup()

		err := emitPart(mpw, h, contentType, body, footer)

		assert.NilError(t, err)
		boundaryMarker := "--" + mpw.Boundary() + "\r\n"
		assert.Equal(t, boundaryMarker+string(textPart), sb.String())

		assert.NilError(t, mpw.Close()) // ensure end boundary written
		contentReader := strings.NewReader(sb.String())
		partReader := newPartReader(contentReader, mpw.Boundary())
		assertNextPart(t, partReader, "text/plain", decodedTextContent)
	})

	t.Run("ReturnsCreatePartError", func(t *testing.T) {
		ew, h, mpw := setupErrWriter("CreatePart error")
		ew.errorOn = "--" + mpw.Boundary()

		err := emitPart(mpw, h, contentType, body, footer)

		assert.Error(t, err, "CreatePart error")
	})

	t.Run("ReturnsWriteError", func(t *testing.T) {
		ew, h, mpw := setupErrWriter("Write error")
		ew.errorOn = "This is only a test." // appears in body

		err := emitPart(mpw, h, contentType, body, footer)

		assert.Error(t, err, "Write error")
	})

	t.Run("ReturnsWriteQuotedPrintableError", func(t *testing.T) {
		ew, h, mpw := setupErrWriter("writeQuotedPrintable error")
		ew.errorOn = "Unsubscribe: " // appears in footer

		err := emitPart(mpw, h, contentType, body, footer)

		assert.Error(t, err, "writeQuotedPrintable error")
	})
}

var instantiatedHtmlFooter = []byte("\r\n" +
	"<p><a href=\"https://foo.com/email/unsubscribe/" +
	"subscriber@foo.com/00000000-1111-2222-3333-444444444444\">" +
	"Unsubscribe</a></p>\r\n" +
	"<p>This footer is over 76 characters wide, " +
	"but will be quoted-printable encoded by EmitMessage.</p>\r\n" +
	"</body></html>")

var encodedHtmlFooter = []byte("\r\n" +
	"<p><a href=3D\"https://foo.com/email/unsubscribe/" +
	"subscriber@foo.com/00000000=\r\n" +
	"-1111-2222-3333-444444444444\">Unsubscribe</a></p>\r\n" +
	"<p>This footer is over 76 characters wide, " +
	"but will be quoted-printable enc=\r\n" +
	"oded by EmitMessage.</p>\r\n" +
	"</body></html>")

var htmlPart = "Content-Transfer-Encoding: quoted-printable\r\n" +
	"Content-Type: " + htmlContentType + "\r\n" +
	"\r\n" +
	string(testTemplate.htmlBody) +
	string(encodedHtmlFooter)

func multipartContent(boundary string) string {
	lines := []string{
		"Content-Type: multipart/alternative; boundary=" + boundary,
		"",
		"--" + boundary,
		textPart,
		"--" + boundary,
		htmlPart,
		"--" + boundary + "--\r\n",
	}
	return strings.Join(lines, "\r\n")
}

var decodedHtmlContent = string(convertToCrlf(testMessage.HtmlBody)) +
	string(instantiatedHtmlFooter)

func parseMultipartMessageAndBoundary(
	t *testing.T, content string,
) (msg *mail.Message, boundary string, partReader *multipart.Reader) {
	t.Helper()

	msg = parseMessage(t, content)
	params := assertContentTypeAndGetParams(
		t, textproto.MIMEHeader(msg.Header), "multipart/alternative",
	)
	boundary = params["boundary"]
	partReader = newPartReader(msg.Body, boundary)
	return
}

func TestEmitMultipart(t *testing.T) {
	setup := func() (*strings.Builder, *writer, *Subscriber) {
		sb := &strings.Builder{}
		sub := newTestSubscriber()
		sub.SetUnsubscribeInfo(testUnsubEmail, testUnsubBaseUrl)
		return sb, &writer{buf: sb}, sub
	}

	t.Run("Succeeds", func(t *testing.T) {
		sb, w, sub := setup()

		testTemplate.emitMultipart(w, sub)

		assert.NilError(t, w.err)
		_, boundary, pr := parseMultipartMessageAndBoundary(t, sb.String())
		assert.Equal(t, multipartContent(boundary), sb.String())
		assertNextPart(t, pr, "text/plain", decodedTextContent)
		assertNextPart(t, pr, "text/html", decodedHtmlContent)
	})
}

func TestMessage(t *testing.T) {
	t.Run("EmitsPlaintextMessage", func(t *testing.T) {
		t.Skip("unimplemented")
	})

	t.Run("EmitsMultipartMessage", func(t *testing.T) {
		t.Skip("pause")
		mt := NewMessageTemplate(testMessage)
		buf := &strings.Builder{}
		sub := *testSubscriber
		sub.SetUnsubscribeInfo(
			"unsubscribe@foo.com", "https://foo.com/email/unsubscribe/",
		)

		err := mt.EmitMessage(buf, &sub)
		assert.NilError(t, err)

		msg := buf.String()
		_, err = mail.ReadMessage(strings.NewReader(msg))
		assert.NilError(t, err)
		assert.Assert(t, strings.HasSuffix(msg, "\r\n"))
		assert.Equal(t, msg, "")
	})
}
