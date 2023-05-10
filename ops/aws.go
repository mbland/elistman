package ops

import (
	"errors"
	"fmt"

	"github.com/aws/smithy-go"
)

// Inspired by:
// https://aws.github.io/aws-sdk-go-v2/docs/handling-errors/#api-error-responses
func AwsError(err error) error {
	var apiErr smithy.APIError

	if errors.As(err, &apiErr) && apiErr.ErrorFault() == smithy.FaultServer {
		return fmt.Errorf("%w: %w", ErrExternal, err)
	}
	return err
}
