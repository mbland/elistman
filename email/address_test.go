//go:build small_tests || all_tests

package email

import (
	"testing"

	"gotest.tools/assert"
)

func newValidator() *AddressValidatorImpl {
	return &AddressValidatorImpl{}
}

func TestValidateBasicEmail(t *testing.T) {
	v := newValidator()

	assert.NilError(t, v.ValidateAddress("mbland@acm.org"))
}
