//go:build small_tests || all_tests

package ops

import (
	"net/mail"
	"net/url"
	"testing"

	"github.com/mbland/elistman/testdata"
	"gotest.tools/assert"
)

const (
	baseUrl = "https://foo.com/email"

	// I originally tried to have the email domain be in IPv6 format a la
	// RFC 5321: Simple Mail Transfer Protocol §4.1.3. I got the idea from a
	// couple of Stack Overflow discussions:
	// - https://www.rfc-editor.org/rfc/rfc5321#section-4.1.3
	// - What characters are allowed in an email address?
	//   https://stackoverflow.com/q/2049502
	// - IPv6 address as the domain portion of an email address
	//   https://stackoverflow.com/q/18128697
	//
	// The required brackets and colons would further demonstrate whether or not
	// the code under test was URI- or query-encoding the address properly.
	//
	// However, mail.ParseAddress will not accept '[', ']', or ':' characters in
	// the domain part. They fall under "specials" in RFC 5322: Internet Message
	// Format §3.2.3. The underlying logic of mail.ParseAddress expressly
	// forbids specials in the domain part of the address:
	// - https://www.rfc-editor.org/rfc/rfc5322#section-3.2.3
	// - https://cs.opensource.google/go/go/+/refs/tags/go1.20.5:src/net/mail/message.go
	//
	// Also, in RFC 5322 §3.4.1, "dtext" explicitly excludes '[' and ']':
	// - https://www.rfc-editor.org/rfc/rfc5322#section-3.4.1
	//
	// It seems the RFC 5322 §3.4.1 "addr-spec" and RFC 5321 §4.1.2 "Mailbox"
	// are very similar, but subtly differ on the matter of domain literals.
	// That said, maybe it's for the best, as Wikipedia suggests:
	//
	// > ...the domain may be an IP address literal, surrounded by square
	// > brackets [], such as jsmith@[192.168.2.1] or jsmith@[IPv6:2001:db8::1],
	// > although this is rarely seen except in email spam.
	// >
	// > - https://en.wikipedia.org/wiki/Email_address#Domain
	//
	// What a mess.
	email = "foo/bar+baz&quux@xyzzy.com"

	uriEncodedEmail   = "foo%2Fbar+baz&quux@xyzzy.com"
	queryEncodedEmail = "foo%2Fbar%2Bbaz%26quux%40xyzzy.com"
	uidStr            = testdata.TestUidStr
)

var uid = testdata.TestUid

func TestTestEmailAddressAndEncodings(t *testing.T) {
	parsed, err := mail.ParseAddress(email)
	assert.NilError(t, err)
	assert.Equal(t, email, parsed.Address)
	assert.Equal(t, uriEncodedEmail, url.PathEscape(email))
	assert.Equal(t, queryEncodedEmail, url.QueryEscape(email))
}

func TestApiEndpoints(t *testing.T) {
	expectedUrl := func(prefix string) string {
		return baseUrl + prefix + uriEncodedEmail + "/" + uidStr
	}

	t.Run("VerifyUrl", func(t *testing.T) {
		assert.Equal(
			t, expectedUrl(ApiPrefixVerify), VerifyUrl(baseUrl, email, uid),
		)
	})

	t.Run("VerifyUrlTrimsBaseUrlTrailingSlash", func(t *testing.T) {
		assert.Equal(
			t, expectedUrl(ApiPrefixVerify), VerifyUrl(baseUrl+"/", email, uid),
		)
	})

	t.Run("UnsubscribeUrl", func(t *testing.T) {
		assert.Equal(
			t,
			expectedUrl(ApiPrefixUnsubscribe),
			UnsubscribeUrl(baseUrl, email, uid))
	})

	t.Run("UnsubscribeMailto", func(t *testing.T) {
		const unsubEmail = "unsubscribe@foo.com"
		const expected = "mailto:" + unsubEmail +
			"?subject=" + queryEncodedEmail + "%20" + uidStr

		assert.Equal(t, expected, UnsubscribeMailto(unsubEmail, email, uid))
	})
}
