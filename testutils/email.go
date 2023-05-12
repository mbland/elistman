package testutils

import (
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

var CharsetUtf8 = map[string]string{"charset": "utf-8"}

func ParseMessage(t *testing.T, content string) (msg *mail.Message) {
	t.Helper()
	var err error

	if msg, err = mail.ReadMessage(strings.NewReader(content)); err != nil {
		t.Fatalf("couldn't parse message from content: %s\n%s", err, content)
	}
	return
}

func AssertValue(t *testing.T, name, expected, actual string) {
	t.Helper()

	if expected != actual {
		t.Fatalf("expected %s: %s, actual: %s", name, expected, actual)
	}
}

func AssertContentTypeAndGetParams(
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
		AssertValue(t, "media type", expectedMediaType, mediaType)
	}
	return
}

func AssertContentType(
	t *testing.T,
	headers textproto.MIMEHeader,
	expectedMediaType string,
	expectedParams map[string]string,
) {
	t.Helper()

	params := AssertContentTypeAndGetParams(t, headers, expectedMediaType)
	const assertMsg = "unexpected Content-Type params"
	assert.Assert(t, is.DeepEqual(expectedParams, params), assertMsg)
}

func GetDecodedContent(t *testing.T, content io.Reader) string {
	t.Helper()
	var decoded []byte
	var err error

	if decoded, err = io.ReadAll(content); err != nil {
		t.Fatalf("couldn't read and decode content: %s", err)
	}
	return string(decoded)
}

func AssertDecodedContent(t *testing.T, content io.Reader, expected string) {
	t.Helper()
	actual := GetDecodedContent(t, content)
	assert.Equal(t, expected, actual)
}

func ParseTextMessage(
	t *testing.T, content string,
) (msg *mail.Message, qpReader *quotedprintable.Reader) {
	t.Helper()

	msg = ParseMessage(t, content)
	header := textproto.MIMEHeader(msg.Header)
	AssertContentType(t, header, "text/plain", CharsetUtf8)

	const cte = "Content-Transfer-Encoding"
	AssertValue(t, cte, "quoted-printable", header.Get(cte))

	qpReader = quotedprintable.NewReader(msg.Body)
	return
}

func GetNextPart(
	t *testing.T, reader *multipart.Reader, mediaType string,
) io.Reader {
	t.Helper()

	var part *multipart.Part
	var err error

	if part, err = reader.NextPart(); err != nil {
		t.Fatalf("couldn't parse message part: %s", err)
	}
	AssertContentType(t, part.Header, mediaType, CharsetUtf8)

	// Per: https://pkg.go.dev/mime/multipart#Reader.NextPart
	//
	// > As a special case, if the "Content-Transfer-Encoding" header has a
	// > value of "quoted-printable", that header is instead hidden and the body
	// > is transparently decoded during Read calls.
	const cte = "Content-Transfer-Encoding"
	AssertValue(t, cte, "", part.Header.Get(cte))
	return part
}

func GetNextPartContent(
	t *testing.T, reader *multipart.Reader, mediaType string,
) string {
	t.Helper()
	return GetDecodedContent(t, GetNextPart(t, reader, mediaType))
}

func AssertNextPart(
	t *testing.T, reader *multipart.Reader, mediaType, decoded string,
) {
	t.Helper()
	AssertDecodedContent(t, GetNextPart(t, reader, mediaType), decoded)
}

func ParseMultipartMessageAndBoundary(
	t *testing.T, content string,
) (msg *mail.Message, boundary string, partReader *multipart.Reader) {
	t.Helper()

	msg = ParseMessage(t, content)
	params := AssertContentTypeAndGetParams(
		t, textproto.MIMEHeader(msg.Header), "multipart/alternative",
	)
	boundary = params["boundary"]
	partReader = multipart.NewReader(msg.Body, boundary)
	return
}

type TestHeader struct {
	mail.Header
}

func (th *TestHeader) Assert(t *testing.T, name string, expected string) {
	t.Helper()

	if actual := th.Get(name); actual != expected {
		t.Errorf("expected %s header: %s, actual: %s", name, expected, actual)
	}
}
