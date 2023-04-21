//go:build small_tests || all_tests

package email

import (
	"testing"

	"gotest.tools/assert"
)

func TestParseUsernameAndHost(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		user, host, err := parseUsernameAndDomain("mbland@acm.org")

		assert.NilError(t, err)
		assert.Equal(t, "mbland", user)
		assert.Equal(t, "acm.org", host)
	})

	t.Run("FailsIfNoAtSign", func(t *testing.T) {
		user, host, err := parseUsernameAndDomain("mblandATacm.org")

		assert.Equal(t, "", user)
		assert.Equal(t, "", host)
		assert.ErrorContains(t, err, `invalid email address: mblandATacm.org`)
		assert.ErrorContains(t, err, `missing '@'`)
	})
}

func TestValidateBasicEmail(t *testing.T) {
	v := ProdAddressValidator{}

	assert.NilError(t, v.ValidateAddress("mbland@acm.org"))
}
