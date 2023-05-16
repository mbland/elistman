//go:build small_tests || all_tests

package email

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mbland/elistman/ops"
	tu "github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestMessageJsonMarshaling(t *testing.T) {
	msg := &Message{}

	assert.NilError(t, json.Unmarshal([]byte(ExampleMessageJson), msg))

	assert.Equal(t, "Foo Bar <foobar@example.com>", msg.From)
	assert.Equal(t, "Test object", msg.Subject)
	assert.Equal(t, "Hello, World!", msg.TextBody)
	assert.Equal(t, "Unsubscribe: "+UnsubscribeUrlTemplate, msg.TextFooter)
	const htmlBody = "<!DOCTYPE html><html><head></head>" +
		"<body>Hello, World!<br/>"
	assert.Equal(t, htmlBody, msg.HtmlBody)
	const htmlFooter = "<a href='" + UnsubscribeUrlTemplate +
		"'>Unsubscribe</a></body></html>"
	assert.Equal(t, htmlFooter, msg.HtmlFooter)
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
		ew := &tu.ErrWriter{
			Buf: sb, ErrorOn: "bar", Err: errors.New("test error"),
		}
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

	t.Run("ReturnsInputLenAfterErrToAvoidIoErrShortWrite", func(t *testing.T) {
		// From: https://pkg.go.dev/io#pkg-variables
		//
		// > ErrShortWrite means that a write accepted fewer bytes than
		// > requested but failed to return an explicit error.
		//
		// This bug was originally surfaced by TestEmitMessage/
		// ReturnsWriteErrors.
		sb, w := setup()
		w.err = errors.New("make subsequent callers think Write succeeded")
		const msg = "Hello, World!"

		n, err := w.Write([]byte(msg))

		assert.NilError(t, err)
		assert.Equal(t, "", sb.String())
		assert.Equal(t, len(msg), n)
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
	setup := func() (*strings.Builder, *tu.ErrWriter) {
		sb := &strings.Builder{}
		return sb, &tu.ErrWriter{Buf: sb}
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
		ew.ErrorOn = "trigger an artificial Write error"
		ew.Err = errors.New("Write error")

		assert.Error(t, writeQuotedPrintable(ew, []byte(msg)), "Write error")
	})

	t.Run("ReturnsCloseError", func(t *testing.T) {
		_, ew := setup()
		msg := "Close will fail when it calls flush on this short message."
		ew.ErrorOn = "Close will fail"
		ew.Err = errors.New("Close error")

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

func TestMessageValidate(t *testing.T) {
	newTestMessage := func() *Message {
		return &Message{
			From:       testMessage.From,
			Subject:    testMessage.Subject,
			TextBody:   testMessage.TextBody,
			TextFooter: testMessage.TextFooter,
			HtmlBody:   testMessage.HtmlBody,
			HtmlFooter: testMessage.HtmlFooter,
		}
	}

	t.Run("Succeeds", func(t *testing.T) {
		assert.NilError(t, newTestMessage().Validate())
	})

	t.Run("EmptyMessageFails", func(t *testing.T) {
		expectedErrMsg := strings.Join(
			[]string{
				"message failed validation: missing From",
				"missing Subject",
				"missing TextBody",
				"missing TextFooter",
			},
			"\n",
		)

		assert.Error(t, (&Message{}).Validate(), expectedErrMsg)
	})

	t.Run("FailsIfFromAddressFailsToParse", func(t *testing.T) {
		msg := newTestMessage()
		msg.From = "not an address"

		const expectedErrMsg = "message failed validation: " +
			"failed to parse From address \"not an address\": " +
			"mail: no angle-addr"
		assert.Error(t, msg.Validate(), expectedErrMsg)
	})

	t.Run("FailsIfHtmlBodyWithoutHtmlFooter", func(t *testing.T) {
		msg := newTestMessage()
		msg.HtmlFooter = ""

		const expectedErrMsg = "message failed validation: HtmlFooter missing"
		assert.Error(t, msg.Validate(), expectedErrMsg)
	})

	t.Run("FailsIfFootersMissingUnsubscribeTemplate", func(t *testing.T) {
		msg := newTestMessage()
		msg.TextFooter = "no unsubscribe template"
		msg.HtmlFooter = "no unsubscribe template"

		expectedErrMsg := strings.Join(
			[]string{
				"message failed validation: " +
					"TextFooter does not contain " + UnsubscribeUrlTemplate,
				"HtmlFooter does not contain " + UnsubscribeUrlTemplate,
			},
			"\n",
		)

		assert.Error(t, msg.Validate(), expectedErrMsg)
	})

	t.Run("FailsIfHtmlFooterWithoutHtmlBody", func(t *testing.T) {
		msg := newTestMessage()
		msg.HtmlBody = ""
		expectedErrMsg := "message failed validation: " +
			"HtmlFooter present, but HtmlBody missing"

		assert.Error(t, msg.Validate(), expectedErrMsg)
	})

	t.Run("FailsIfMessageValidatorFuncReturnsError", func(t *testing.T) {
		msg := newTestMessage()
		msg.From = "Foo Bar <foo@bar.com>"
		okFunc := func(*Message, string, string) error {
			return nil
		}
		errFunc := func(m *Message, n, a string) error {
			return fmt.Errorf(
				"testing From: \"%s <%s>\" Subject: %s", n, a, m.Subject,
			)
		}
		expectedErrorMsg := "message failed validation: " +
			"testing From: \"" + msg.From + "\" Subject: " + msg.Subject

		assert.Error(t, msg.Validate(okFunc, errFunc), expectedErrorMsg)
	})
}

func byteStringsEqual(t *testing.T, expected, actual []byte) {
	t.Helper()
	assert.Check(t, is.Equal(string(expected), string(actual)))
}

func TestNewMessageTemplate(t *testing.T) {
	assertMessageTemplatesEqual := func(
		t *testing.T, expected, actual *MessageTemplate,
	) {
		t.Helper()

		byteStringsEqual(t, expected.from, actual.from)
		byteStringsEqual(t, expected.subject, actual.subject)
		byteStringsEqual(t, expected.textBody, actual.textBody)
		byteStringsEqual(t, expected.textFooter, actual.textFooter)
		byteStringsEqual(t, expected.htmlBody, actual.htmlBody)
		byteStringsEqual(t, expected.htmlFooter, actual.htmlFooter)
	}

	t.Run("Succeeds", func(t *testing.T) {
		mt := NewMessageTemplate(testMessage)

		assertMessageTemplatesEqual(t, testTemplate, mt)
	})

	t.Run("WillAddANewlineToEndOfBodiesIfNeeded", func(t *testing.T) {
		mt := NewMessageTemplate(&Message{
			From:       testMessage.From,
			Subject:    testMessage.Subject,
			TextBody:   strings.TrimRight(testMessage.TextBody, "\r\n"),
			TextFooter: testMessage.TextFooter,
			HtmlBody:   strings.TrimRight(testMessage.HtmlBody, "\r\n"),
			HtmlFooter: testMessage.HtmlFooter,
		})

		assertMessageTemplatesEqual(t, testTemplate, mt)
	})
}

var testRecipient *Recipient = &Recipient{
	Email: "subscriber@foo.com",
	Uid:   uuid.MustParse(testUid),
}

func newTestRecipient() *Recipient {
	var sub Recipient = *testRecipient
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

func TestEmitTextOnly(t *testing.T) {
	setup := func() (*strings.Builder, *writer, *tu.ErrWriter, *Recipient) {
		sb := &strings.Builder{}
		sub := newTestRecipient()
		sub.SetUnsubscribeInfo(testUnsubEmail, testApiBaseUrl)
		return sb, &writer{buf: sb}, &tu.ErrWriter{Buf: sb}, sub
	}

	t.Run("Succeeds", func(t *testing.T) {
		sb, w, _, sub := setup()

		testTemplate.emitTextOnly(w, sub)

		assert.NilError(t, w.err)
		assert.Equal(t, textOnlyContent, sb.String())
		_, qpr := tu.ParseTextMessage(t, sb.String())
		tu.AssertDecodedContent(t, qpr, decodedTextContent)
	})

	t.Run("ReturnsWriteQuotedPrintableError", func(t *testing.T) {
		_, w, ew, sub := setup()
		w.buf = ew
		ew.ErrorOn = "Unsubscribe: "
		ew.Err = errors.New("writeQuotedPrintable error")

		testTemplate.emitTextOnly(w, sub)

		assert.Error(t, w.err, "writeQuotedPrintable error")
	})
}

var textPart string = "Content-Transfer-Encoding: quoted-printable\r\n" +
	"Content-Type: " + textContentType + "\r\n" +
	"\r\n" +
	string(testTemplate.textBody) +
	string(encodedTextFooter)

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
		*tu.ErrWriter, textproto.MIMEHeader, *multipart.Writer,
	) {
		sb, h, _ := setup()
		ew := &tu.ErrWriter{Buf: sb, Err: errors.New(errorMsg)}
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
		partReader := multipart.NewReader(contentReader, mpw.Boundary())
		tu.AssertNextPart(t, partReader, "text/plain", decodedTextContent)
	})

	t.Run("ReturnsCreatePartError", func(t *testing.T) {
		ew, h, mpw := setupErrWriter("CreatePart error")
		ew.ErrorOn = "--" + mpw.Boundary()

		err := emitPart(mpw, h, contentType, body, footer)

		assert.Error(t, err, "CreatePart error")
	})

	t.Run("ReturnsWriteError", func(t *testing.T) {
		ew, h, mpw := setupErrWriter("Write error")
		ew.ErrorOn = "This is only a test." // appears in body

		err := emitPart(mpw, h, contentType, body, footer)

		assert.Error(t, err, "Write error")
	})

	t.Run("ReturnsWriteQuotedPrintableError", func(t *testing.T) {
		ew, h, mpw := setupErrWriter("writeQuotedPrintable error")
		ew.ErrorOn = "Unsubscribe: " // appears in footer

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

func TestEmitMultipart(t *testing.T) {
	setup := func() (*strings.Builder, *writer, *Recipient) {
		sb := &strings.Builder{}
		sub := newTestRecipient()
		sub.SetUnsubscribeInfo(testUnsubEmail, testApiBaseUrl)
		return sb, &writer{buf: sb}, sub
	}

	setupWithError := func(
		errMsg string,
	) (*writer, *tu.ErrWriter, *Recipient) {
		sb, w, sub := setup()
		ew := &tu.ErrWriter{Buf: sb, Err: errors.New(errMsg)}
		w.buf = ew
		return w, ew, sub
	}

	t.Run("Succeeds", func(t *testing.T) {
		sb, w, sub := setup()

		testTemplate.emitMultipart(w, sub)

		assert.NilError(t, w.err)
		_, boundary, pr := tu.ParseMultipartMessageAndBoundary(t, sb.String())
		assert.Equal(t, multipartContent(boundary), sb.String())
		tu.AssertNextPart(t, pr, "text/plain", decodedTextContent)
		tu.AssertNextPart(t, pr, "text/html", decodedHtmlContent)
	})

	t.Run("ReturnTextPartError", func(t *testing.T) {
		w, ew, sub := setupWithError("text/plain part error")
		ew.ErrorOn = "Content-Type: text/plain"

		testTemplate.emitMultipart(w, sub)

		assert.Error(t, w.err, "text/plain part error")
	})

	t.Run("ReturnHtmlPartError", func(t *testing.T) {
		w, ew, sub := setupWithError("text/html part error")
		ew.ErrorOn = "Content-Type: text/html"

		testTemplate.emitMultipart(w, sub)

		assert.Error(t, w.err, "text/html part error")
	})

	t.Run("ReturnCloseError", func(t *testing.T) {
		w, ew, sub := setupWithError("multipart.Writer.Close error")
		ew.ErrorOn = "--\r\n" // end of the final multipart boundary marker

		testTemplate.emitMultipart(w, sub)

		assert.Error(t, w.err, "multipart.Writer.Close error")
	})
}

const testUnsubHeaderValue = "<mailto:" + testUnsubEmail +
	"?subject=subscriber@foo.com%20" + testUid + ">, " +
	"<" + testApiBaseUrl + ops.ApiPrefixUnsubscribe +
	"subscriber@foo.com/" + testUid + ">"

const expectedHeaders = "From: EListMan@foo.com\r\n" +
	"To: subscriber@foo.com\r\n" +
	"Subject: This is a test\r\n" +
	"List-Unsubscribe: " + testUnsubHeaderValue + "\r\n" +
	"List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n" +
	"MIME-Version: 1.0\r\n"

func assertMessageHeaders(t *testing.T, msg *mail.Message, content string) {
	t.Helper()

	th := tu.TestHeader{Header: msg.Header}
	th.Assert(t, "From", testMessage.From)
	th.Assert(t, "To", testRecipient.Email)
	th.Assert(t, "Subject", testMessage.Subject)
	th.Assert(t, "List-Unsubscribe", testUnsubHeaderValue)
	th.Assert(t, "List-Unsubscribe-Post", "List-Unsubscribe=One-Click")
	th.Assert(t, "MIME-Version", "1.0")
}

func TestEmitMessage(t *testing.T) {
	setup := func() (*strings.Builder, *writer, *Recipient) {
		sb := &strings.Builder{}
		sub := newTestRecipient()
		sub.SetUnsubscribeInfo(testUnsubEmail, testApiBaseUrl)
		return sb, &writer{buf: sb}, sub
	}

	setupWithError := func(
		errMsg string,
	) (*writer, *tu.ErrWriter, *Recipient) {
		sb, w, sub := setup()
		ew := &tu.ErrWriter{Buf: sb, Err: errors.New(errMsg)}
		w.buf = ew
		return w, ew, sub
	}

	t.Run("EmitsPlaintextMessage", func(t *testing.T) {
		sb, w, sub := setup()
		textTemplate := *testTemplate
		textTemplate.htmlBody = []byte{}

		err := textTemplate.EmitMessage(w, sub)

		assert.NilError(t, err)
		content := sb.String()
		msg, qpr := tu.ParseTextMessage(t, content)
		assert.Equal(t, expectedHeaders+textOnlyContent, content)
		assertMessageHeaders(t, msg, content)
		tu.AssertDecodedContent(t, qpr, decodedTextContent)
	})

	t.Run("EmitsMultipartMessage", func(t *testing.T) {
		sb, w, sub := setup()

		err := testTemplate.EmitMessage(w, sub)

		assert.NilError(t, err)
		content := sb.String()
		msg, boundary, pr := tu.ParseMultipartMessageAndBoundary(t, content)
		assert.Equal(t, expectedHeaders+multipartContent(boundary), content)
		assertMessageHeaders(t, msg, content)
		tu.AssertNextPart(t, pr, "text/plain", decodedTextContent)
		tu.AssertNextPart(t, pr, "text/html", decodedHtmlContent)
	})

	t.Run("ReturnsWriteErrors", func(t *testing.T) {
		w, ew, sub := setupWithError("write MIME-Version error")
		ew.ErrorOn = "MIME-Version"

		err := testTemplate.EmitMessage(w, sub)

		expected := "error emitting message to " + sub.Email +
			": write MIME-Version error"
		assert.Error(t, err, expected)
	})
}

func TestNewListMessageTemplateFromJson(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		mt, err := NewListMessageTemplateFromJson([]byte(ExampleMessageJson))

		assert.NilError(t, err)
		assert.Assert(t, mt != nil)
	})

	t.Run("ErrorsIfParsingJsonFails", func(t *testing.T) {
		mt, err := NewListMessageTemplateFromJson(
			[]byte("{ \"definitely not proper JSON\": foobar}"),
		)

		assert.Assert(t, is.Nil(mt))
		const expectedMsg = "failed to parse message input from JSON"
		assert.ErrorContains(t, err, expectedMsg)
	})

	t.Run("ErrorsIfValidationFails", func(t *testing.T) {
		mt, err := NewListMessageTemplateFromJson([]byte("{}"))

		assert.Assert(t, is.Nil(mt))
		assert.ErrorContains(t, err, "message failed validation: ")
	})
}
