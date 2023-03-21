package email

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
)

type Mailer interface {
	Send(
		toAddr string,
		fromAddr string,
		subject string,
		textMsg string,
		htmlMsg string,
	) error
}

type SesMailer struct {
	Client *ses.Client
}

func NewSesMailer(awsConfig aws.Config) *SesMailer {
	return &SesMailer{Client: ses.NewFromConfig(awsConfig)}
}

func (mailer SesMailer) Send(
	toAddr string,
	fromAddr string,
	subject string,
	textMsg string,
	htmlMsg string,
) error {
	return nil
}
