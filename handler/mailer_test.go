package handler

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
)

func newTestMailer() *SesMailer {
	cfg := aws.Config{}
	return NewSesMailer(cfg)
}

// This function will be replaced by more substantial tests once I begin to
// implement SesMailer.
func TestMailerInitialization(t *testing.T) {
	mailer := newTestMailer()

	assert.NotNil(t, mailer)
}
