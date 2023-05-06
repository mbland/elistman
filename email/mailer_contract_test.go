//go:build medium_tests || contract_tests || all_tests

package email

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func setupSesMailer(
	cfg aws.Config,
) (*SesMailer, *testutils.Logs, string, context.Context) {

	logs := &testutils.Logs{}
	mailer := &SesMailer{
		Client:    ses.NewFromConfig(cfg),
		ClientV2:  sesv2.NewFromConfig(cfg),
		ConfigSet: "unused-for-now",
		Log:       logs.NewLogger(),
	}
	email := testutils.RandomEmail(4, "elistman-test.com")
	return mailer, logs, email, context.Background()
}

func TestSesMailer(t *testing.T) {
	cfg, err := testutils.LoadDefaultAwsConfig()

	assert.NilError(t, err)

	t.Run("SuppressorInterface", func(t *testing.T) {
		mailer, logs, email, ctx := setupSesMailer(cfg)

		assert.Assert(t, !mailer.IsSuppressed(ctx, email))

		err := mailer.Suppress(ctx, email)
		assert.NilError(t, err)

		assert.Assert(t, mailer.IsSuppressed(ctx, email))

		err = mailer.Unsuppress(ctx, email)
		assert.NilError(t, err)

		assert.Assert(t, !mailer.IsSuppressed(ctx, email))

		const expectedEmptyLogs = ""
		assert.Equal(t, expectedEmptyLogs, logs.Logs())
	})
}
