package ops

type OperationResult int

const (
	Invalid OperationResult = iota
	AlreadySubscribed
	VerifyLinkSent
	Subscribed
	NotSubscribed
	Unsubscribed
)

func (r OperationResult) String() string {
	switch r {
	case Invalid:
		return "Invalid"
	case AlreadySubscribed:
		return "Already subscribed"
	case VerifyLinkSent:
		return "Verify link sent"
	case Subscribed:
		return "Subscribed"
	case NotSubscribed:
		return "Not subscribed"
	case Unsubscribed:
		return "Unsubscribed"
	}
	return "Unknown"
}

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
