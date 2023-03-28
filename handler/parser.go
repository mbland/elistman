package handler

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
)

const (
	SubscribePrefix   = "/subscribe/"
	VerifyPrefix      = "/verify/"
	UnsubscribePrefix = "/unsubscribe/"
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

func parseApiEvent(
	endpoint string, params map[string]string,
) (*eventOperation, error) {
	if optype, err := parseOperationType(endpoint); err != nil {
		return nil, err
	} else if email, err := parseEmailParam(endpoint, params); err != nil {
		return nil, err
	} else if uid, err := parseUidParam(optype, endpoint, params); err != nil {
		return nil, err
	} else {
		return &eventOperation{Type: optype, Email: email, Uid: uid}, nil
	}
}

func parseOperationType(endpoint string) (eventOperationType, error) {
	if strings.HasPrefix(endpoint, SubscribePrefix) {
		return SubscribeOp, nil
	} else if strings.HasPrefix(endpoint, VerifyPrefix) {
		return VerifyOp, nil
	} else if strings.HasPrefix(endpoint, UnsubscribePrefix) {
		return UnsubscribeOp, nil
	}
	return UndefinedOp, fmt.Errorf("unexpected endpoint: %s", endpoint)
}

func parseEmailParam(
	endpoint string, params map[string]string,
) (string, error) {
	if emailParam, ok := params["email"]; !ok {
		return "", fmt.Errorf("missing email parameter: %s", endpoint)
	} else if email, err := mail.ParseAddress(emailParam); err != nil {
		return "", fmt.Errorf("invalid email address: %s: %s", emailParam, err)
	} else {
		return email.Address, nil
	}
}

func parseUidParam(
	optype eventOperationType, endpoint string, params map[string]string,
) (uuid.UUID, error) {
	if optype == SubscribeOp {
		return uuid.Nil, nil
	} else if uidParam, ok := params["uid"]; !ok {
		return uuid.Nil, fmt.Errorf("missing uid parameter: %s", endpoint)
	} else if uid, err := uuid.Parse(uidParam); err != nil {
		return uuid.Nil, fmt.Errorf("invalid uid: %s: %s", uidParam, err)
	} else {
		return uid, nil
	}
}

func parseMailtoEvent(
	froms, tos []string, unsubscribeRecipient, subject string,
) (*eventOperation, error) {
	if err := checkMailAddresses(froms, tos, unsubscribeRecipient); err != nil {
		return nil, err
	} else if email, uid, err := parseEmailSubject(subject); err != nil {
		return nil, err
	} else {
		return &eventOperation{Type: UnsubscribeOp, Email: email, Uid: uid}, nil
	}
}

func checkMailAddresses(
	froms, tos []string, unsubscribeRecipient string,
) error {
	if len(froms) != 1 {
		return fmt.Errorf(
			"more than one From address: %s", strings.Join(froms, ","),
		)
	} else if len(tos) != 1 {
		return fmt.Errorf(
			"more than one To address: %s", strings.Join(tos, ","),
		)
	} else if to := tos[0]; to != unsubscribeRecipient {
		return fmt.Errorf("not addressed to %s: %s", unsubscribeRecipient, to)
	}
	return nil
}

func parseEmailSubject(subject string) (string, uuid.UUID, error) {
	params := strings.Split(subject, " ")
	if len(params) != 2 || params[0] == "" || params[1] == "" {
		return "", uuid.Nil, fmt.Errorf(
			"subject not in `<email> <uid>` format: %s", subject,
		)
	} else if email, err := mail.ParseAddress(params[0]); err != nil {
		return "", uuid.Nil, fmt.Errorf(
			"invalid email address: %s: %s", params[0], err,
		)
	} else if uid, err := uuid.Parse(params[1]); err != nil {
		return "", uuid.Nil, fmt.Errorf("invalid uid: %s: %s", params[1], err)
	} else {
		return email.Address, uid, nil
	}
}
