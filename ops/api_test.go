//go:build small_tests || all_tests

package ops

import (
	"testing"

	"github.com/mbland/elistman/testdata"
	"gotest.tools/assert"
)

const TestEmail = "foo@bar.com"
const TestUidStr = "00000000-1111-2222-3333-444444444444"

func TestApiEndpoints(t *testing.T) {
	const baseUrl = "https://foo.com/email"

	t.Run("VerifyUrl", func(t *testing.T) {
		const expected = baseUrl + ApiPrefixVerify +
			testdata.TestEmail + "/" + testdata.TestUidStr

		actual := VerifyUrl(baseUrl, testdata.TestEmail, testdata.TestUid)

		assert.Equal(t, expected, actual)
	})

	t.Run("VerifyUrlTrimsBaseUrlTrailingSlash", func(t *testing.T) {
		const expected = baseUrl + ApiPrefixVerify +
			testdata.TestEmail + "/" + testdata.TestUidStr

		actual := VerifyUrl(baseUrl+"/", testdata.TestEmail, testdata.TestUid)

		assert.Equal(t, expected, actual)
	})

	t.Run("UnsubscribeUrl", func(t *testing.T) {
		const expected = baseUrl + ApiPrefixUnsubscribe +
			testdata.TestEmail + "/" + testdata.TestUidStr

		actual := UnsubscribeUrl(
			baseUrl, testdata.TestEmail, testdata.TestUid,
		)

		assert.Equal(t, expected, actual)
	})

	t.Run("UnsubscribeMailto", func(t *testing.T) {
		const unsubEmail = "unsubscribe@foo.com"
		const expected = "mailto:" + unsubEmail + "?subject=" +
			testdata.TestEmail + "%20" + testdata.TestUidStr

		actual := UnsubscribeMailto(
			unsubEmail, testdata.TestEmail, testdata.TestUid,
		)

		assert.Equal(t, expected, actual)
	})
}
