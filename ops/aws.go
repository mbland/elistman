package ops

import (
	"errors"
	"fmt"

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
