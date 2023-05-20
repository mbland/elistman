package ops

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go"
)

// Inspired by:
// https://aws.github.io/aws-sdk-go-v2/docs/handling-errors/#api-error-responses
func AwsError(prefix string, err error) error {
	var apiErr smithy.APIError

	if errors.As(err, &apiErr) && apiErr.ErrorFault() == smithy.FaultServer {
		return fmt.Errorf("%w: %s: %s", ErrExternal, prefix, err)
	}
	return fmt.Errorf("%s: %s", prefix, err)
}

func LoadDefaultAwsConfig() (cfg aws.Config, err error) {
	if cfg, err = config.LoadDefaultConfig(context.Background()); err != nil {
		err = fmt.Errorf("failed to load AWS config: %s", err)
	}
	return
}
