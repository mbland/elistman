package ops

//go:generate go run golang.org/x/tools/cmd/stringer -type=OperationResult
type OperationResult int

const (
	Invalid OperationResult = iota
	AlreadySubscribed
	VerifyLinkSent
	Subscribed
	NotSubscribed
	Unsubscribed
)

type OperationErrorInternal struct {
	Message string
}

func (err *OperationErrorInternal) Error() string {
	return err.Message
}

type OperationErrorExternal struct {
	Message string
}

func (err *OperationErrorExternal) Error() string {
	return err.Message
}
