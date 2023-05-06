package testutils

import (
	"log"
	"strings"
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestLogger() (*strings.Builder, *log.Logger) {
	builder := &strings.Builder{}
	logger := log.New(builder, "test logger: ", 0)
	return builder, logger
}

type LogFixture interface {
	Logs() string
}

func AssertLogsContain(t *testing.T, lf LogFixture, message string) {
	t.Helper()
	assert.Assert(t, is.Contains(lf.Logs(), message))
}
