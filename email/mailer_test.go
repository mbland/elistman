package email

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"gotest.tools/assert"
)

func newTestMailer() *SesMailer {
	cfg := aws.Config{}
	return NewSesMailer(cfg)
}

// This function will be replaced by more substantial tests once I begin to
// implement SesMailer.
func TestMailerInitialization(t *testing.T) {
	mailer := newTestMailer()

	assert.Assert(t, mailer != nil)
}
