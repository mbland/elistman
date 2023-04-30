package email

import (
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestConvertToCrlfAddsCarriageFeedBeforeNewlineAsNeeded(t *testing.T) {
	checkCrlfOutput := func(t *testing.T, before, expected string) {
		t.Helper()
		actual := string(convertToCrlf(before))
		assert.Check(t, is.Equal(expected, actual))
	}

	checkCrlfOutput(t, "", "")
	checkCrlfOutput(t, "\n", "\r\n")
	checkCrlfOutput(t, "\r", "\r")
	checkCrlfOutput(t, "foo\nbar\nbaz\n", "foo\r\nbar\r\nbaz\r\n")
	checkCrlfOutput(t, "foo\r\nbar\r\nbaz", "foo\r\nbar\r\nbaz")
	checkCrlfOutput(t, "foo\r\nbar\nbaz", "foo\r\nbar\r\nbaz")
	checkCrlfOutput(t, "foo\rbar\nbaz", "foo\rbar\r\nbaz")
}
