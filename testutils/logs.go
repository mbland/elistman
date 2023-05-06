package testutils

import (
	"log"
	"strings"
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

type Logs struct {
	Builder strings.Builder
}

func NewLogs() (*Logs, *log.Logger) {
	logs := &Logs{}
	return logs, logs.NewLogger()
}

func (tl *Logs) NewLogger() *log.Logger {
	return log.New(&tl.Builder, "test logger: ", 0)
}

func (tl *Logs) AssertContains(t *testing.T, message string) {
	t.Helper()
	assert.Assert(t, is.Contains(tl.Builder.String(), message))
}

func (tl *Logs) Logs() string {
	return tl.Builder.String()
}

func (tl *Logs) Reset() {
	tl.Builder.Reset()
}
