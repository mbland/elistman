package email

// This validate-before-sending algorithm:
// - https://mailtrap.io/blog/verify-email-address-without-sending/
//
// Can be implemented using:
// - https://pkg.go.dev/net/smtp

type AddressValidator interface {
	ValidateAddress(addr string) error
}

type AddressValidatorImpl struct{}

func (AddressValidatorImpl) ValidateAddress(addr string) error {
	return nil
}
