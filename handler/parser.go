package handler

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"net/url"
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
	var info *opInfo = nil

	if optype, err := parseOperationType(req.RawPath); err != nil {
		return nil, &ParseError{optype, err.Error()}
	} else if err := parseParams(req); err != nil {
		return nil, &ParseError{optype, err.Error()}
	} else {
		info = &opInfo{optype, req.Params}
	}

	if email, err := info.parseEmail(); err != nil {
		return nil, &ParseError{info.Type, err.Error()}
	} else if uid, err := info.parseUid(); err != nil {
		return nil, &ParseError{info.Type, err.Error()}
	} else {
		return &eventOperation{info.Type, email, uid}, nil
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
	return UndefinedOp, fmt.Errorf("unknown endpoint: %s", endpoint)
}

func parseParams(req *apiRequest) error {
	if req.Method != http.MethodPost {
		return nil
	}

	values, err := parseBody(req.ContentType, req.Body)

	if err != nil {
		const errFmt = `failed to parse body params with Content-Type %q: %s`
		return fmt.Errorf(errFmt, req.ContentType, err)
	}

	for k, v := range values {
		if len(v) != 1 {
			values := strings.Join(v, ", ")
			return fmt.Errorf("multiple values for %q: %s", k, values)
		} else if _, exists := req.Params[k]; !exists {
			req.Params[k] = v[0]
		}
	}
	return nil
}

func parseBody(contentType, body string) (url.Values, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)

	if err != nil {
		const errFormat = "failed to parse %q: %s"
		return url.Values{}, fmt.Errorf(errFormat, contentType, err)
	}

	switch mediaType {
	case "application/x-www-form-urlencoded":
		return url.ParseQuery(body)
	case "multipart/form-data":
		return parseFormData(body, params)
	}
	return url.Values{}, fmt.Errorf("unknown media type: %s", mediaType)
}

func parseFormData(body string, params map[string]string) (url.Values, error) {
	reader := multipart.NewReader(strings.NewReader(body), params["boundary"])
	values := url.Values{}

	for {
		if part, err := reader.NextPart(); err == io.EOF {
			break
		} else if err != nil {
			return url.Values{}, err
		} else if data, err := io.ReadAll(part); err != nil {
			return url.Values{}, err
		} else {
			values.Add(part.FormName(), string(data))
		}
	}
	return values, nil
}

type opInfo struct {
	Type   eventOperationType
	Params map[string]string
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
		return nilValue, fmt.Errorf("missing %s parameter", name)
	} else if v, err := parse(value); err != nil {
		e := fmt.Errorf("invalid %s parameter: %s: %s", name, value, err)
		return nilValue, e
	} else {
		return v, nil
	}
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
