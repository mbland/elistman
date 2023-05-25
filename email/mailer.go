package email

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/mbland/elistman/ops"
)

type Mailer interface {
	Send(
		ctx context.Context, recipient string, msg []byte,
	) (messageId string, err error)
}

type SesMailer struct {
	Client    SesV2Api
	ConfigSet string
}

func (mailer *SesMailer) Send(
	ctx context.Context, recipient string, msg []byte,
) (messageId string, err error) {
	sesMsg := &sesv2.SendEmailInput{
		ConfigurationSetName: aws.String(mailer.ConfigSet),
		Content: &sestypes.EmailContent{
			Raw: &sestypes.RawMessage{Data: msg},
		},
		Destination: &sestypes.Destination{
			ToAddresses: []string{recipient},
		},
	}
	var output *sesv2.SendEmailOutput

	if output, err = mailer.Client.SendEmail(ctx, sesMsg); err != nil {
		err = ops.AwsError("send to "+recipient+" failed", err)
	} else {
		messageId = aws.ToString(output.MessageId)
	}
	return
}
