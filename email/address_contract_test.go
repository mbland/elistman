//go:build medium_tests || contract_tests || all_tests

package email

import (
	"context"
	"flag"
	"net"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
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
	cfg, err := testutils.LoadDefaultAwsConfig()
	assert.NilError(t, err)

	suppressor := &SesSuppressor{sesv2.NewFromConfig(cfg)}
	v := ProdAddressValidator{suppressor, net.DefaultResolver}
	ctx := context.Background()

	failure, err := v.ValidateAddress(ctx, goodEmailAddress)

	assert.NilError(t, err)
	assert.Assert(t, is.Nil(failure))
}
