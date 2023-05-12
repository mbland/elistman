//go:build small_tests || all_tests

package ops

import (
	"testing"

	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestApiEndpoints(t *testing.T) {
	const baseUrl = "https://foo.com/email"

	t.Run("VerifyUrl", func(t *testing.T) {
		const expected = baseUrl + ApiPrefixVerify +
			testutils.TestEmail + "/" + testutils.TestUidStr

		actual := VerifyUrl(baseUrl, testutils.TestEmail, testutils.TestUid)

		assert.Equal(t, expected, actual)
	})

	t.Run("VerifyUrlTrimsBaseUrlTrailingSlash", func(t *testing.T) {
		const expected = baseUrl + ApiPrefixVerify +
			testutils.TestEmail + "/" + testutils.TestUidStr

		actual := VerifyUrl(baseUrl+"/", testutils.TestEmail, testutils.TestUid)

		assert.Equal(t, expected, actual)
	})

	t.Run("UnsubscribeUrl", func(t *testing.T) {
		const expected = baseUrl + ApiPrefixUnsubscribe +
			testutils.TestEmail + "/" + testutils.TestUidStr

		actual := UnsubscribeUrl(
			baseUrl, testutils.TestEmail, testutils.TestUid,
		)

		assert.Equal(t, expected, actual)
	})

	t.Run("UnsubscribeMailto", func(t *testing.T) {
		const unsubEmail = "unsubscribe@foo.com"
		const expected = "mailto:" + unsubEmail + "?subject=" +
			testutils.TestEmail + "%20" + testutils.TestUidStr

		actual := UnsubscribeMailto(
			unsubEmail, testutils.TestEmail, testutils.TestUid,
		)

		assert.Equal(t, expected, actual)
	})
}
