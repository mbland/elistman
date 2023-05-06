//go:build medium_tests || contract_tests || all_tests

package email

import (
	"context"
	"testing"

	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
)

func TestSesMailer(t *testing.T) {
	cfg, err := testutils.LoadDefaultAwsConfig()

	assert.NilError(t, err)

	t.Run("SuppressorInterface", func(t *testing.T) {
		mailer, logs := setupSesMailer(cfg, "unused-config-set")
		email := testutils.RandomEmail(4, "elistman-test.com")
		ctx := context.Background()

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
