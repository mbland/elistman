package email

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/mbland/elistman/ops"
)

type Mailer interface {
	Send(
		ctx context.Context, recipient string, msg []byte,
	) (messageId string, err error)
}

type SesMailer struct {
	Client    SesApi
	ConfigSet string
}

func (mailer *SesMailer) Send(
	ctx context.Context, recipient string, msg []byte,
) (messageId string, err error) {
	sesMsg := &ses.SendRawEmailInput{
		Destinations:         []string{recipient},
		ConfigurationSetName: aws.String(mailer.ConfigSet),
		RawMessage:           &sestypes.RawMessage{Data: msg},
	}
	var output *ses.SendRawEmailOutput

	if output, err = mailer.Client.SendRawEmail(ctx, sesMsg); err != nil {
		err = ops.AwsError("send to "+recipient+" failed", err)
	} else {
		messageId = aws.ToString(output.MessageId)
	}
	return
}
