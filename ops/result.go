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
