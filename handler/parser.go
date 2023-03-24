package handler

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
)

const (
	SubcribePrefix    = "/subscribe/"
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

func parseApiRequestOperation(
	endpoint string, params map[string]string,
) (*eventOperation, error) {
	if optype, err := parseOperationTypeFromEndpoint(endpoint); err != nil {
		return nil, err
	} else if email, err := parseEmailFromPathParam(endpoint, params); err != nil {
		return nil, err
	} else if uid, err := parseUidFromPathParam(optype, endpoint, params); err != nil {
		return nil, err
	} else {
		return &eventOperation{Type: optype, Email: email, Uid: uid}, nil
	}
}

func parseOperationTypeFromEndpoint(
	endpoint string,
) (eventOperationType, error) {
	if strings.HasPrefix(endpoint, SubcribePrefix) {
		return SubscribeOp, nil
	} else if strings.HasPrefix(endpoint, VerifyPrefix) {
		return VerifyOp, nil
	} else if strings.HasPrefix(endpoint, UnsubscribePrefix) {
		return UnsubscribeOp, nil
	}
	return UndefinedOp, fmt.Errorf("unexpected endpoint: %s", endpoint)
}

func parseEmailFromPathParam(
	endpoint string, params map[string]string,
) (string, error) {
	if emailParam, ok := params["email"]; !ok {
		return "", fmt.Errorf("missing email path parameter: %s", endpoint)
	} else if email, err := mail.ParseAddress(emailParam); err != nil {
		return "", fmt.Errorf(
			"failed to parse email path parameter: %s: %s", emailParam, err,
		)
	} else {
		return email.Address, nil
	}
}

func parseUidFromPathParam(
	optype eventOperationType, endpoint string, params map[string]string,
) (uuid.UUID, error) {
	if optype == SubscribeOp {
		return uuid.Nil, nil
	} else if uidParam, ok := params["uid"]; !ok {
		return uuid.Nil, fmt.Errorf("missing uid path parameter: %s", endpoint)
	} else if uid, err := uuid.Parse(uidParam); err != nil {
		return uuid.Nil, fmt.Errorf(
			"failed to parse uid path parameter: %s: %s", uidParam, err,
		)
	} else {
		return uid, nil
	}
}

func parseMailtoEventOperation(
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
			"more than one To: address: %s", strings.Join(tos, ","),
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
	} else if email, err := parseEmailFromSubject(params[0]); err != nil {
		return "", uuid.Nil, err
	} else if uid, err := uuid.Parse(params[1]); err != nil {
		return "", uuid.Nil, fmt.Errorf(
			"invalid uid in subject: %s: %s", params[1], err,
		)
	} else {
		return email, uid, nil
	}
}

func parseEmailFromSubject(emailParam string) (string, error) {
	if email, err := mail.ParseAddress(emailParam); err != nil {
		return "", fmt.Errorf(
			"failed to parse email from subject: %s: %s", emailParam, err,
		)
	} else {
		return email.Address, nil
	}
}
