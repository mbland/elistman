package handler

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
)

const (
	SubscribePrefix   = "/subscribe"
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
	Type    eventOperationType
	Message string
}

func (e *ParseError) Error() string {
	return e.Type.String() + ": " + e.Message
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

type apiRequest struct {
	RawPath     string
	Method      string
	ContentType string
	Params      map[string]string
	Body        string
}

func parseApiRequest(req *apiRequest) (*eventOperation, error) {
	if pi, err := newOpInfo(req.RawPath, req.Params); err != nil {
		return nil, err
	} else if email, err := pi.parseEmail(); err != nil {
		return nil, err
	} else if uid, err := pi.parseUid(); err != nil {
		return nil, err
	} else {
		return &eventOperation{pi.Type, email, uid}, nil
	}
}

type opInfo struct {
	Type   eventOperationType
	Params map[string]string
}

func newOpInfo(endpoint string, params map[string]string) (*opInfo, error) {
	if optype, err := parseOperationType(endpoint); err != nil {
		return nil, err
	} else {
		return &opInfo{optype, params}, nil
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
		Type: UndefinedOp, Message: "unknown endpoint: " + endpoint,
	}
}

func (oi *opInfo) parseEmail() (string, error) {
	return parseParam(oi, "email", "", parseEmailAddress)
}

func (oi *opInfo) parseUid() (uuid.UUID, error) {
	if oi.Type == SubscribeOp {
		return uuid.Nil, nil
	}
	return parseParam(oi, "uid", uuid.Nil, uuid.Parse)
}

func parseEmailAddress(emailParam string) (string, error) {
	if email, err := mail.ParseAddress(emailParam); err != nil {
		return "", err
	} else {
		return email.Address, nil
	}
}

func parseParam[T string | uuid.UUID](
	oi *opInfo, name string, nilValue T, parse func(string) (T, error),
) (T, error) {
	if value, ok := oi.Params[name]; !ok {
		return nilValue, oi.parseError("missing " + name + " parameter")
	} else if v, err := parse(value); err != nil {
		msg := fmt.Sprintf("invalid %s parameter: %s: %s", name, value, err)
		return nilValue, oi.parseError(msg)
	} else {
		return v, nil
	}
}

func (oi *opInfo) parseError(message string) error {
	return &ParseError{Type: oi.Type, Message: message}
}

type parsedSubject struct {
	Email string
	Uid   uuid.UUID
}

var nilSubject *parsedSubject = &parsedSubject{}

type mailtoEvent struct {
	From         []string
	To           []string
	Subject      string
	MessageId    string
	SpfVerdict   string
	DkimVerdict  string
	SpamVerdict  string
	VirusVerdict string
	DmarcVerdict string
	DmarcPolicy  string
}

func parseMailtoEvent(
	ev *mailtoEvent, unsubscribeAddr string,
) (*eventOperation, error) {
	if err := checkMailAddresses(ev.From, ev.To, unsubscribeAddr); err != nil {
		return nil, err
	} else if subject, err := parseEmailSubject(ev.Subject); err != nil {
		return nil, err
	} else {
		return &eventOperation{UnsubscribeOp, subject.Email, subject.Uid}, nil
	}
}

func checkMailAddresses(froms, tos []string, unsubscribeAddr string) error {
	if err := checkForOnlyOneAddress("From", froms); err != nil {
		return err
	} else if err := checkForOnlyOneAddress("To", tos); err != nil {
		return err
	} else if to := tos[0]; to != unsubscribeAddr {
		return fmt.Errorf("not addressed to %s: %s", unsubscribeAddr, to)
	}
	return nil
}

func checkForOnlyOneAddress(headerName string, addrs []string) error {
	if len(addrs) == 0 {
		return fmt.Errorf("missing %s address", headerName)
	} else if len(addrs) != 1 {
		errFormat := "more than one %s address: %s"
		return fmt.Errorf(errFormat, headerName, strings.Join(addrs, ","))
	}
	return nil
}

func parseEmailSubject(subject string) (*parsedSubject, error) {
	params := strings.Split(subject, " ")
	if len(params) != 2 || params[0] == "" || params[1] == "" {
		const errFormat = "subject not in `<email> <uid>` format: \"%s\""
		return nilSubject, fmt.Errorf(errFormat, subject)
	} else if email, err := parseEmailAddress(params[0]); err != nil {
		const errFormat = "invalid email address: %s: %s"
		return nilSubject, fmt.Errorf(errFormat, params[0], err)
	} else if uid, err := uuid.Parse(params[1]); err != nil {
		return nilSubject, fmt.Errorf("invalid uid: %s: %s", params[1], err)
	} else {
		return &parsedSubject{email, uid}, nil
	}
}
