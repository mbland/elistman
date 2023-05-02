package email

import (
	"errors"
	"fmt"
	"net/mail"
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

var testSubscriber *Subscriber = &Subscriber{
	Email: "subscriber@foo.com",
	Uid:   uuid.MustParse("00000000-1111-2222-3333-444444444444"),
}

func TestMessage(t *testing.T) {
	origMsg := &Message{
		From:       "EListMan@foo.com",
		Subject:    "This is a test",
		TextBody:   "This is only a test.",
		TextFooter: "\n\nUnsubscribe: " + UnsubscribeUrlTemplate,

		// Ensure this is longer than 76 chars so we can see the quoted-printable
		// encoding kicking in.
		HtmlBody: `<!DOCTYPE html>` +
			`<html><head><title>This is a test</title></head>` +
			`<body><p>This is only a test.</p>`,
		HtmlFooter: fmt.Sprintf(
			"\n\n<p><a href=\"%s\">Unsubscribe</a></p></body></html>",
			UnsubscribeUrlTemplate,
		),
	}

	t.Run("Succeeds", func(t *testing.T) {
		mt := NewMessageTemplate(origMsg)
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
