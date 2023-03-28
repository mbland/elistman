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

func (t eventOperationType) String() string {
	switch t {
	case UndefinedOp:
		return "Undefined"
	case SubscribeOp:
		return "Subscribe"
	case VerifyOp:
		return "Verify"
	case UnsubscribeOp:
		return "Unsubscribe"
	}
	return "Unknown"
}

type eventOperation struct {
	Type  eventOperationType
	Email string
	Uid   uuid.UUID
}

type ParseError struct {
	Type     eventOperationType
	Endpoint string
	Message  string
}

func (e *ParseError) Error() string {
	return e.Type.String() + ": " + e.Message + ": " + e.Endpoint
}

func (e *ParseError) Is(target error) bool {
	// Inspired by the example from the "Customizing error tests with Is and As
	// methods" section of https://go.dev/blog/go1.13-errors.
	if t, ok := target.(*ParseError); !ok {
		return false
	} else {
		return e.Type == t.Type || t.Type == UndefinedOp
	}
}

func parseApiEvent(
	endpoint string, params map[string]string,
) (*eventOperation, error) {
	if pi, err := newPathInfo(endpoint, params); err != nil {
		return nil, err
	} else if email, err := pi.parseEmail(); err != nil {
		return nil, err
	} else if uid, err := pi.parseUid(); err != nil {
		return nil, err
	} else {
		return &eventOperation{pi.Type, email, uid}, nil
	}
}

type pathInfo struct {
	Type     eventOperationType
	Endpoint string
	Params   map[string]string
}

func newPathInfo(endpoint string, params map[string]string) (*pathInfo, error) {
	if optype, err := parseOperationType(endpoint); err != nil {
		return nil, err
	} else {
		return &pathInfo{optype, endpoint, params}, nil
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
	return UndefinedOp, &ParseError{
		Type: UndefinedOp, Message: "unknown endpoint", Endpoint: endpoint,
	}
}

func (pi *pathInfo) parseEmail() (string, error) {
	return parsePathParam(pi, "email", "", parseEmailAddress)
}

func (pi *pathInfo) parseUid() (uuid.UUID, error) {
	if pi.Type == SubscribeOp {
		return uuid.Nil, nil
	}
	return parsePathParam(pi, "uid", uuid.Nil, uuid.Parse)
}

func parseEmailAddress(emailParam string) (string, error) {
	if email, err := mail.ParseAddress(emailParam); err != nil {
		return "", err
	} else {
		return email.Address, nil
	}
}

func parsePathParam[T string | uuid.UUID](
	pi *pathInfo, name string, nilValue T, parse func(string) (T, error),
) (T, error) {
	if value, ok := pi.Params[name]; !ok {
		return nilValue, pi.parseError("missing " + name + " parameter")
	} else if v, err := parse(value); err != nil {
		msg := fmt.Sprintf("invalid %s parameter: %s: %s", name, value, err)
		return nilValue, pi.parseError(msg)
	} else {
		return v, nil
	}
}

func (pi *pathInfo) parseError(message string) error {
	return &ParseError{Type: pi.Type, Endpoint: pi.Endpoint, Message: message}
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
	} else if email, err := parseEmailAddress(params[0]); err != nil {
		return "", uuid.Nil, fmt.Errorf(
			"invalid email address: %s: %s", params[0], err,
		)
	} else if uid, err := uuid.Parse(params[1]); err != nil {
		return "", uuid.Nil, fmt.Errorf("invalid uid: %s: %s", params[1], err)
	} else {
		return email, uid, nil
	}
}
