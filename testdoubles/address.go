package testdoubles

import (
	"context"
	"testing"

	"gotest.tools/assert"
)

type AddressValidator struct {
	Email string
	Error error
}

func NewAddressValidator() *AddressValidator {
	return &AddressValidator{}
}

func (av *AddressValidator) ValidateAddress(
	ctx context.Context, email string,
) error {
	av.Email = email
	return av.Error
}

func (av *AddressValidator) AssertValidated(
	t *testing.T, expectedEmail string,
) {
	t.Helper()

	assert.Equal(t, expectedEmail, av.Email)
}
