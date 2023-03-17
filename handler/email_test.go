package handler

import (
	"testing"

	"gotest.tools/assert"
)

func newValidator() *EmailValidator {
	return &EmailValidator{}
}

func TestValidateBasicEmail(t *testing.T) {
	v := newValidator()

	assert.NilError(t, v.ValidateAddress("mbland@acm.org"))
}
