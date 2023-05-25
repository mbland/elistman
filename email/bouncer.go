package email

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/mbland/elistman/ops"
)

type Bouncer interface {
	Bounce(
		ctx context.Context,
		emailDomain,
		messageId string,
		recipients []string,
		timestamp time.Time,
	) (string, error)
}

type SesBouncer struct {
	Client SesApi
}

// https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-lambda-example-functions.html
func (mailer *SesBouncer) Bounce(
	ctx context.Context,
	emailDomain,
	messageId string,
	recipients []string,
	timestamp time.Time,
) (bounceMessageId string, err error) {
	recipientInfo := make([]sestypes.BouncedRecipientInfo, len(recipients))

	for i, recipient := range recipients {
		recipientInfo[i].Recipient = aws.String(recipient)
		recipientInfo[i].BounceType = sestypes.BounceTypeContentRejected
	}

	input := &ses.SendBounceInput{
		BounceSender:      aws.String("mailer-daemon@" + emailDomain),
		OriginalMessageId: aws.String(messageId),
		MessageDsn: &sestypes.MessageDsn{
			ReportingMta: aws.String("dns; " + emailDomain),
			ArrivalDate:  aws.Time(timestamp.Truncate(time.Second)),
		},
		Explanation: aws.String(
			"Unauthenticated email is not accepted due to " +
				"the sending domain's DMARC policy.",
		),
		BouncedRecipientInfoList: recipientInfo,
	}
	var output *ses.SendBounceOutput

	if output, err = mailer.Client.SendBounce(ctx, input); err != nil {
		err = ops.AwsError("sending bounce failed", err)
	} else {
		bounceMessageId = aws.ToString(output.MessageId)
	}
	return
}
