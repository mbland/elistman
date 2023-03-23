package handler

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

type eventOperationType int

const (
	UndefinedOp eventOperationType = iota
	SubscribeOp
	VerifyOp
	UnsubscribeOp
)

type eventOperation struct {
	Type  eventOperationType
	Email string
	Uid   uuid.UUID
}

func parseApiRequestOperation(
	endpoint string, params map[string]string,
) (*eventOperation, error) {
	op := eventOperation{}

	if strings.HasPrefix(endpoint, "/subscribe/") {
		op.Type = SubscribeOp
	} else if strings.HasPrefix(endpoint, "/verify/") {
		op.Type = VerifyOp
	} else if strings.HasPrefix(endpoint, "/unsubscribe/") {
		op.Type = UnsubscribeOp
	} else {
		return nil, fmt.Errorf("unexpected endpoint: %s", endpoint)
	}

	if escapedEmail, ok := params["email"]; !ok {
		return nil, fmt.Errorf("missing email path parameter: %s", endpoint)
	} else if email, err := url.PathUnescape(escapedEmail); err != nil {
		return nil, fmt.Errorf(
			"failed to unescape email path parameter: %s: %s", escapedEmail, err,
		)
	} else {
		op.Email = email
	}

	if op.Type == SubscribeOp {
		return &op, nil
	}

	if uidParam, ok := params["uid"]; !ok {
		return nil, fmt.Errorf("missing uid path parameter: %s", endpoint)
	} else if uid, err := uuid.Parse(uidParam); err != nil {
		return nil, fmt.Errorf("failed to parse uid path parameter: %s", err)
	} else {
		op.Uid = uid
	}
	return &op, nil
}

func parseMailtoEventOperation(
	froms, tos []string, unsubscribe_recipient, subject string,
) (*eventOperation, error) {
	op := eventOperation{}

	if len(froms) != 1 {
		return nil, fmt.Errorf(
			"too many From addresses: %s", strings.Join(froms, ","),
		)
	} else if len(tos) != 1 {
		return nil, fmt.Errorf(
			"too many To: addresses: %s", strings.Join(tos, ","),
		)
	} else if to := tos[0]; to != unsubscribe_recipient {
		return nil, fmt.Errorf(
			"not addressed to %s: %s", unsubscribe_recipient, to,
		)
	} else if params := strings.Split(subject, " "); len(params) != 2 {
		return nil, fmt.Errorf(
			"subject not in `<email> <uid>` format: %s", subject,
		)
	} else if email, err := url.QueryUnescape(params[0]); err != nil {
		return nil, fmt.Errorf(
			"failed to unescape email from subject: %s: %s", params[0], err,
		)
	} else if uid, err := uuid.Parse(params[1]); err != nil {
		return nil, fmt.Errorf("invalid uid in subject: %s: %s", params[1], err)
	} else {
		op.Type = UnsubscribeOp
		op.Email = email
		op.Uid = uid
	}
	return &op, nil
}
