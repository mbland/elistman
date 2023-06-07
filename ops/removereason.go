package ops

type RemoveReason string

const (
	RemoveReasonNil       RemoveReason = ""
	RemoveReasonBounce    RemoveReason = "Bounce"
	RemoveReasonComplaint RemoveReason = "Complaint"
)
