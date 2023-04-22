//go:build medium_tests || contract_tests || all_tests

package email

import (
	"context"
	"flag"
	"net"
	"testing"

	"gotest.tools/assert"
)

var goodEmailAddress string

func init() {
	flag.StringVar(
		&goodEmailAddress,
		"good-email",
		"mbland@acm.org",
		"A known good email address that will pass domain validation via DNS",
	)
}

func TestValidateAddressSucceedsUsingLiveDnsService(t *testing.T) {
	// When SesMailer is ready, replace TestSuppressor with it.
	v := ProdAddressValidator{&TestSuppressor{}, net.DefaultResolver}
	ctx := context.Background()

	assert.NilError(t, v.ValidateAddress(ctx, goodEmailAddress))
}
