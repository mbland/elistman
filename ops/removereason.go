package ops

type RemoveReason string

const (
	RemoveReasonBounce    RemoveReason = "Bounce"
	RemoveReasonComplaint RemoveReason = "Complaint"
)
