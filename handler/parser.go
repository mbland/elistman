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
	Type     eventOperationType
	Email    string
	Uid      uuid.UUID
	OneClick bool
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
	RequestId   string
	RawPath     string
	Method      string
	ContentType string
	Params      map[string]string
	Body        string
}

func parseApiRequest(req *apiRequest) (*eventOperation, error) {
	if optype, err := parseOperationType(req.RawPath); err != nil {
		return parseError(optype, err)
	} else if params, err := parseParams(req); err != nil {
		return parseError(optype, err)
	} else if email, err := parseEmail(params); err != nil {
		return parseError(optype, err)
	} else if uid, err := parseUid(optype, params); err != nil {
		return parseError(optype, err)
	} else {
		return &eventOperation{
			optype,
			email,
			uid,
			isOneClickUnsubscribeRequest(optype, req, params),
		}, nil
	}
}

func parseError(optype eventOperationType, err error) (*eventOperation, error) {
	return nil, &ParseError{optype, err.Error()}
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

func parseParams(req *apiRequest) (map[string]string, error) {
	values := url.Values{}
	params := map[string]string{}
	var err error = nil

	if req.Method == http.MethodPost {
		if values, err = parseBody(req.ContentType, req.Body); err != nil {
			errFmt := `failed to parse body params with content-type %q: %s`
			return params, fmt.Errorf(errFmt, req.ContentType, err)
		}
	} else if req.Body != "" {
		return params, fmt.Errorf("nonempty body for HTTP %s", req.Method)
	}

	for k, v := range values {
		if len(v) != 1 {
			values := strings.Join(v, ", ")
			err = fmt.Errorf("multiple values for %q: %s", k, values)
			return map[string]string{}, err
		} else if pathV, ok := req.Params[k]; ok {
			errFormat := "path and body parameters defined for %q: %s, %s"
			err = fmt.Errorf(errFormat, k, pathV, v[0])
			return map[string]string{}, err
		}
		params[k] = v[0]
	}

	for k, v := range req.Params {
		params[k] = v
	}
	return params, nil
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

func parseEmail(params map[string]string) (string, error) {
	return parseParam(params, "email", "", parseEmailAddress)
}

func parseUid(
	optype eventOperationType, params map[string]string,
) (uuid.UUID, error) {
	if optype == SubscribeOp {
		return uuid.Nil, nil
	}
	return parseParam(params, "uid", uuid.Nil, uuid.Parse)
}

func parseEmailAddress(emailParam string) (email string, err error) {
	if email, err := mail.ParseAddress(emailParam); err != nil {
		return "", err
	} else {
		return email.Address, nil
	}
}

func parseParam[T string | uuid.UUID](
	params map[string]string,
	name string,
	nilValue T,
	parse func(string) (T, error),
) (T, error) {
	if value, ok := params[name]; !ok {
		return nilValue, fmt.Errorf("missing %s parameter", name)
	} else if v, err := parse(value); err != nil {
		e := fmt.Errorf("invalid %s parameter: %s: %s", name, value, err)
		return nilValue, e
	} else {
		return v, nil
	}
}

func isOneClickUnsubscribeRequest(
	optype eventOperationType, req *apiRequest, params map[string]string,
) bool {
	// See the file comments in email/mailer.go for references describing the
	// one click unsubscribe mechanism.
	return optype == UnsubscribeOp &&
		req.Method == http.MethodPost &&
		params["List-Unsubscribe"] == "One-Click"
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
		return &eventOperation{
			UnsubscribeOp, subject.Email, subject.Uid, true,
		}, nil
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

func checkForOnlyOneAddress(headerName string, addrs []string) (err error) {
	if len(addrs) == 0 {
		err = fmt.Errorf("missing %s address", headerName)
	} else if len(addrs) != 1 {
		errFormat := "more than one %s address: %s"
		err = fmt.Errorf(errFormat, headerName, strings.Join(addrs, ","))
	}
	return
}

func parseEmailSubject(subject string) (result *parsedSubject, err error) {
	result = &parsedSubject{}
	params := strings.Split(subject, " ")
	if len(params) != 2 || params[0] == "" || params[1] == "" {
		err = fmt.Errorf(`subject not in "<email> <uid>" format: "%s"`, subject)
	} else if email, emailErr := parseEmailAddress(params[0]); emailErr != nil {
		err = fmt.Errorf("invalid email address: %s: %s", params[0], emailErr)
	} else if uid, uidErr := uuid.Parse(params[1]); uidErr != nil {
		err = fmt.Errorf("invalid uid: %s: %s", params[1], uidErr)
	} else {
		result = &parsedSubject{email, uid}
	}
	return
}
