package testdoubles

import (
	"context"
	"testing"

	"github.com/mbland/elistman/email"
	"gotest.tools/assert"
)

type AddressValidator struct {
	Email   string
	Failure *email.ValidationFailure
	Error   error
}

func NewAddressValidator() *AddressValidator {
	return &AddressValidator{}
}

func (av *AddressValidator) ValidateAddress(
	ctx context.Context, email string,
) (*email.ValidationFailure, error) {
	av.Email = email
	return av.Failure, av.Error
}

func (av *AddressValidator) AssertValidated(
	t *testing.T, expectedEmail string,
) {
	t.Helper()

	assert.Equal(t, expectedEmail, av.Email)
}
