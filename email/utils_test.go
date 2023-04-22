package email

type TestSuppressor struct {
	checkedEmail       string
	isSuppressedResult bool
	suppressedEmail    string
	suppressErr        error
}

func (ts *TestSuppressor) IsSuppressed(email string) bool {
	ts.checkedEmail = email
	return ts.isSuppressedResult
}

func (ts *TestSuppressor) Suppress(email string) error {
	ts.suppressedEmail = email
	return ts.suppressErr
}
