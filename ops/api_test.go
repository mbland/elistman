//go:build small_tests || all_tests

package ops

import (
	"testing"

	"github.com/mbland/elistman/testdata"
	"gotest.tools/assert"
)

const (
	baseUrl = "https://foo.com/email"
	email   = testdata.TestEmail
	uidStr  = testdata.TestUidStr
)

var uid = testdata.TestUid

func TestApiEndpoints(t *testing.T) {
	expectedUrl := func(prefix string) string {
		return baseUrl + prefix + email + "/" + uidStr
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
			"?subject=" + email + "%20" + uidStr

		assert.Equal(t, expected, UnsubscribeMailto(unsubEmail, email, uid))
	})
}
