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
