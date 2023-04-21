//go:build small_tests || all_tests

package email

import (
	"context"
	"net"
	"testing"

	"gotest.tools/assert"
)

type TestSuppressor struct {
	returnValue bool
}

func (ts *TestSuppressor) IsSuppressed(email string) bool {
	return ts.returnValue
}

func TestParseAddress(t *testing.T) {
	t.Run("Succeeds", func(t *testing.T) {
		email, user, host, err := parseAddress("mbland@acm.org")

		assert.NilError(t, err)
		assert.Equal(t, "mbland@acm.org", email)
		assert.Equal(t, "mbland", user)
		assert.Equal(t, "acm.org", host)
	})

	t.Run("FailsIfNoAtSign", func(t *testing.T) {
		email, user, host, err := parseAddress("mblandATacm.org")

		assert.Equal(t, "", email)
		assert.Equal(t, "", user)
		assert.Equal(t, "", host)
		assert.ErrorContains(t, err, `invalid email address: mblandATacm.org`)
		assert.ErrorContains(t, err, `missing '@'`)
	})
}

func TestValidateBasicEmail(t *testing.T) {
	v := ProdAddressValidator{&TestSuppressor{}, net.DefaultResolver}

	err := v.ValidateAddress(context.Background(), "mbland@acm.org")

	assert.NilError(t, err)
}
