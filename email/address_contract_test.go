//go:build medium_tests || contract_tests || all_tests

package email

import (
	"context"
	"flag"
	"net"
	"testing"

	"github.com/mbland/elistman/testutils"
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
	cfg, err := testutils.LoadDefaultAwsConfig()
	assert.NilError(t, err)

	mailer, logs := setupSesMailer(cfg, "unused-config-set")
	v := ProdAddressValidator{mailer, net.DefaultResolver}
	ctx := context.Background()

	assert.NilError(t, v.ValidateAddress(ctx, goodEmailAddress))
	const expectedEmptyLogs = ""
	assert.Equal(t, expectedEmptyLogs, logs.Logs())
}
