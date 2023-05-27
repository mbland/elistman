package email

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/mbland/elistman/ops"
)

type Mailer interface {
	BulkCapacityAvailable(ctx context.Context, numToSend int64) error

	Send(
		ctx context.Context, recipient string, msg []byte,
	) (messageId string, err error)
}

type SesMailer struct {
	Client    SesV2Api
	ConfigSet string
	Throttle  Throttle
}

func (mailer *SesMailer) BulkCapacityAvailable(
	ctx context.Context, numToSend int64,
) error {
	return mailer.Throttle.BulkCapacityAvailable(ctx, numToSend)
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

	if err = mailer.Throttle.PauseBeforeNextSend(ctx); err != nil {
		err = fmt.Errorf("send to %s failed: %w", recipient, err)
	} else if output, err = mailer.Client.SendEmail(ctx, sesMsg); err != nil {
		err = ops.AwsError("send to "+recipient+" failed", err)
	} else {
		messageId = aws.ToString(output.MessageId)
	}
	return
}
