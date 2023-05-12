//go:build small_tests || all_tests

package handler

import (
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mbland/elistman/testutils"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

var nilSubject *parsedSubject = &parsedSubject{}

func TestUnknownEventOperationType(t *testing.T) {
	unknownOp := Undefined - 1
	assert.Equal(t, "eventOperationType(-1)", unknownOp.String())
}

func TestEventOperationString(t *testing.T) {
	t.Run("Undefined", func(t *testing.T) {
		op := &eventOperation{}
		assert.Equal(t, "Undefined", op.String())
	})

	t.Run("Subscribe", func(t *testing.T) {
		op := &eventOperation{Type: Subscribe, Email: "mbland@acm.org"}
		assert.Equal(t, "Subscribe: mbland@acm.org", op.String())
	})

	t.Run("Verify", func(t *testing.T) {
		op := &eventOperation{
			Type: Verify, Email: "mbland@acm.org", Uid: testValidUid,
		}

		assert.Equal(t, "Verify: mbland@acm.org "+testValidUidStr, op.String())
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		op := &eventOperation{
			Type: Unsubscribe, Email: "mbland@acm.org", Uid: testValidUid,
		}

		expected := "Unsubscribe: mbland@acm.org " + testValidUidStr
		assert.Equal(t, expected, op.String())
	})

	t.Run("OneClickUnsubscribe", func(t *testing.T) {
		op := &eventOperation{
			Type:     Unsubscribe,
			Email:    "mbland@acm.org",
			Uid:      testValidUid,
			OneClick: true,
		}

		expected := "Unsubscribe (One-Click): mbland@acm.org " + testValidUidStr
		assert.Equal(t, expected, op.String())
	})
}

func TestParseErrorIncludesOptypeAndMessage(t *testing.T) {
	err := &ParseError{
		Type:    Subscribe,
		Message: "invalid email parameter: mbland acm.org",
	}

	assert.Equal(
		t, "Subscribe: invalid email parameter: mbland acm.org", err.Error(),
	)
}

func TestParamError(t *testing.T) {
	t.Run("IsAParseError", func(t *testing.T) {
		op, err := paramError(Verify, errors.New("proper parse error"))
		var parseErr *ParseError

		assert.Assert(t, is.Nil(op))
		assert.Assert(t, errors.As(err, &parseErr))
		assert.DeepEqual(
			t, &ParseError{Verify, "proper parse error"}, parseErr,
		)
	})

	t.Run("IsAUSerInputError", func(t *testing.T) {
		op, err := paramError(Subscribe, errors.New("user input error"))
		var parseErr *ParseError
		assert.Assert(t, is.Nil(op))
		assert.Assert(t, testutils.ErrorIs(err, ErrUserInput))
		assert.Assert(t, !errors.As(err, &parseErr))
	})
}

type testParams map[string]string

func (tp testParams) urlencoded() (string, string) {
	values := url.Values{}

	for k, v := range tp {
		values.Add(k, v)
	}
	return "application/x-www-form-urlencoded", values.Encode()
}

func (tp testParams) formData() (string, string) {
	builder := strings.Builder{}
	writer := multipart.NewWriter(&builder)

	for k, v := range tp {
		writer.WriteField(k, v)
	}
	writer.Close()
	return writer.FormDataContentType(), builder.String()
}

func (tp testParams) urlValues() url.Values {
	values := url.Values{}

	for k, v := range tp {
		values.Add(k, v)
	}
	return values
}

func TestParseFormData(t *testing.T) {
	params := testParams{"email": "mbland@acm.org", "uid": "0123-456-789"}
	contentType, body := params.formData()
	_, mediaParams, err := mime.ParseMediaType(contentType)

	if err != nil {
		t.Fatalf("content-type %q failed to parse: %s", contentType, err)
	}

	t.Run("Success", func(t *testing.T) {
		values, err := parseFormData(body, mediaParams)

		assert.NilError(t, err)
		assert.DeepEqual(t, params.urlValues(), values)
	})

	t.Run("ErrorOnNextPart", func(t *testing.T) {
		badBody := strings.ReplaceAll(body, mediaParams["boundary"], "")

		values, err := parseFormData(badBody, mediaParams)

		assert.ErrorContains(t, err, "multipart: NextPart: EOF")
		assert.DeepEqual(t, url.Values{}, values)
	})

	t.Run("ErrorOnReadAll", func(t *testing.T) {
		badBody := strings.Replace(body, mediaParams["boundary"]+"--", "", 1)

		values, err := parseFormData(badBody, mediaParams)

		assert.Error(t, err, "unexpected EOF")
		assert.DeepEqual(t, url.Values{}, values)
	})
}

func TestParseBody(t *testing.T) {
	params := testParams{"email": "mbland@acm.org", "uid": "0123-456-789"}

	t.Run("ErrorIfMediaTypeFailsToParse", func(t *testing.T) {
		_, body := params.urlencoded()
		values, err := parseBody("foobar/", body)

		expected := `failed to parse "foobar/": ` +
			"mime: expected token after slash"
		assert.ErrorContains(t, err, expected)
		assert.DeepEqual(t, url.Values{}, values)
	})

	t.Run("ErrorIfUnknownMediaType", func(t *testing.T) {
		_, body := params.urlencoded()
		values, err := parseBody("foobar", body)

		assert.ErrorContains(t, err, "unknown media type: foobar")
		assert.DeepEqual(t, url.Values{}, values)
	})

	t.Run("ParseUrlencoded", func(t *testing.T) {
		contentType, body := params.urlencoded()
		values, err := parseBody(contentType, body)

		assert.NilError(t, err)
		assert.DeepEqual(t, params.urlValues(), values)
	})

	t.Run("ParseFormData", func(t *testing.T) {
		contentType, body := params.formData()
		values, err := parseBody(contentType, body)

		assert.NilError(t, err)
		assert.DeepEqual(t, params.urlValues(), values)
	})
}

func TestParseParams(t *testing.T) {
	newRequest := func() *apiRequest {
		return &apiRequest{
			Method:      http.MethodPost,
			ContentType: "application/x-www-form-urlencoded",
			Params:      map[string]string{},
			Body:        "email=mbland%40acm.org&uid=0123-456-789",
		}
	}
	parsedParams := map[string]string{
		"email": "mbland@acm.org", "uid": "0123-456-789",
	}

	t.Run("ErrorIfBodyPresentForNonPostRequest", func(t *testing.T) {
		req := newRequest()
		req.Method = http.MethodGet

		result, err := parseParams(req)

		assert.ErrorContains(t, err, "nonempty body for HTTP "+req.Method)
		assert.DeepEqual(t, map[string]string{}, result)
	})

	t.Run("ParseError", func(t *testing.T) {
		req := newRequest()
		req.Body = "email=mbland@acm.org;uid=0123-456-789"

		result, err := parseParams(req)

		expected := fmt.Sprintf(
			`failed to parse body params with content-type "%s": `,
			req.ContentType,
		)
		assert.ErrorContains(t, err, expected)
		assert.DeepEqual(t, map[string]string{}, result)
	})

	t.Run("Success", func(t *testing.T) {
		req := newRequest()

		result, err := parseParams(req)

		assert.NilError(t, err)
		assert.DeepEqual(t, parsedParams, result)
	})

	t.Run("ErrorIfPathAndBodyParamsBothDefined", func(t *testing.T) {
		req := newRequest()
		req.Params["email"] = "foo@bar.com"

		result, err := parseParams(req)

		expected := `path and body parameters defined for "email": ` +
			"foo@bar.com, mbland@acm.org"
		assert.ErrorContains(t, err, expected)
		assert.DeepEqual(t, map[string]string{}, result)
	})

	t.Run("ErrorIfParamHasMultipleValues", func(t *testing.T) {
		req := newRequest()
		req.Body = "email=mbland%40acm.org&email=foo%40bar.com"

		result, err := parseParams(req)

		expected := `multiple values for "email": mbland@acm.org, foo@bar.com`
		assert.ErrorContains(t, err, expected)
		assert.DeepEqual(t, map[string]string{}, result)
	})
}

func TestParseOperationType(t *testing.T) {
	t.Run("Subscribe", func(t *testing.T) {
		result, err := parseOperationType(ApiPrefixSubscribe)

		assert.NilError(t, err)
		assert.Equal(t, "Subscribe", result.String())
	})

	t.Run("Verify", func(t *testing.T) {
		result, err := parseOperationType(VerifyPrefix + "/foobar")

		assert.NilError(t, err)
		assert.Equal(t, "Verify", result.String())
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		result, err := parseOperationType(UnsubscribePrefix + "/foobar")

		assert.NilError(t, err)
		assert.Equal(t, "Unsubscribe", result.String())
	})

	t.Run("Undefined", func(t *testing.T) {
		result, err := parseOperationType("/foobar/baz")

		assert.Equal(t, "Undefined", result.String())
		assert.ErrorContains(t, err, "unknown endpoint: /foobar/baz")
	})
}

// This series of parseEmail tests serves to test the underlying
// parseEmailAddress and parseParam functions.
func TestParseEmail(t *testing.T) {
	t.Run("ParamMissing", func(t *testing.T) {
		result, err := parseEmail(map[string]string{})

		assert.Equal(t, "", result)
		assert.ErrorContains(t, err, "missing email parameter")
	})

	t.Run("ParamInvalid", func(t *testing.T) {
		result, err := parseEmail(map[string]string{"email": "bazquux"})

		assert.Equal(t, "", result)
		expected := "invalid email parameter: bazquux: " +
			"mail: missing '@' or angle-addr"
		assert.ErrorContains(t, err, expected)
	})

	t.Run("ParamValid", func(t *testing.T) {
		result, err := parseEmail(map[string]string{"email": "mbland@acm.org"})

		assert.NilError(t, err)
		assert.Equal(t, "mbland@acm.org", result)
	})
}

func TestParseUid(t *testing.T) {
	t.Run("IgnoreSubscribeOp", func(t *testing.T) {
		result, err := parseUid(Subscribe, map[string]string{})

		assert.NilError(t, err)
		assert.Equal(t, uuid.Nil, result)
	})

	t.Run("ParamValid", func(t *testing.T) {
		expected, err := uuid.Parse("00000000-1111-2222-3333-444444444444")
		assert.NilError(t, err)

		result, err := parseUid(
			Verify, map[string]string{"uid": expected.String()},
		)

		assert.NilError(t, err)
		assert.Equal(t, expected, result)
	})
}

func TestIsOneClickSubscribeRequest(t *testing.T) {
	postReq := &apiRequest{Method: http.MethodPost}
	oneClickParams := map[string]string{"List-Unsubscribe": "One-Click"}

	t.Run("FalseIfNotAnUnsubscribeOp", func(t *testing.T) {
		result := isOneClickUnsubscribeRequest(
			Subscribe, postReq, oneClickParams,
		)

		assert.Assert(t, result == false)
	})

	t.Run("FalseIfNotAPostRequest", func(t *testing.T) {
		result := isOneClickUnsubscribeRequest(
			Unsubscribe, &apiRequest{Method: http.MethodGet}, oneClickParams,
		)

		assert.Assert(t, result == false)
	})

	t.Run("FalseIfNoOneClickParameter", func(t *testing.T) {
		result := isOneClickUnsubscribeRequest(
			Unsubscribe, postReq, map[string]string{},
		)

		assert.Assert(t, result == false)
	})

	t.Run("True", func(t *testing.T) {
		result := isOneClickUnsubscribeRequest(
			Unsubscribe, postReq, oneClickParams,
		)

		assert.Assert(t, result == true)
	})
}

func TestParseApiRequest(t *testing.T) {
	t.Run("Unknown", func(t *testing.T) {
		var parseError *ParseError

		result, err := parseApiRequest(&apiRequest{
			RawPath: "/foobar", Params: map[string]string{},
		})

		assert.Assert(t, is.Nil(result))
		assert.Assert(t, errors.As(err, &parseError))
		assert.Equal(t, Undefined, parseError.Type)
		assert.ErrorContains(t, err, "unknown endpoint: /foobar")
	})

	t.Run("ErrorWhileParsingParams", func(t *testing.T) {
		var parseError *ParseError

		req := &apiRequest{
			RawPath:     ApiPrefixSubscribe,
			Params:      map[string]string{},
			Method:      http.MethodPost,
			ContentType: "application/x-www-form-urlencoded",
			Body:        "email=mbland%40acm.org&email=foo%40bar.com",
		}

		result, err := parseApiRequest(req)

		assert.Assert(t, is.Nil(result))
		assert.Assert(t, errors.As(err, &parseError))
		assert.Equal(t, Subscribe, parseError.Type)
		assert.ErrorContains(
			t, err, `multiple values for "email": mbland@acm.org, foo@bar.com`,
		)
	})

	t.Run("UserInputForEmailInvalid", func(t *testing.T) {
		result, err := parseApiRequest(&apiRequest{
			RawPath: ApiPrefixSubscribe,
			Params:  map[string]string{"email": "foobar"},
		})

		assert.Assert(t, is.Nil(result))
		assert.Assert(t, testutils.ErrorIs(err, ErrUserInput))
		assert.ErrorContains(t, err, "invalid email parameter: foobar")
	})

	t.Run("PathParameterForUidInvalid", func(t *testing.T) {
		var parseError *ParseError

		result, err := parseApiRequest(&apiRequest{
			RawPath: VerifyPrefix + "mbland@acm.org/0123456789",
			Params: map[string]string{
				"email": "mbland@acm.org", "uid": "0123456789",
			},
		})

		assert.Assert(t, is.Nil(result))
		assert.Assert(t, errors.As(err, &parseError))
		assert.Equal(t, Verify, parseError.Type)
		assert.ErrorContains(t, err, "invalid uid parameter: 0123456789")
	})

	t.Run("SuccessfulSubscribe", func(t *testing.T) {
		req := &apiRequest{
			RawPath:     ApiPrefixSubscribe,
			Params:      map[string]string{},
			Method:      http.MethodPost,
			ContentType: "application/x-www-form-urlencoded",
			Body:        "email=mbland%40acm.org",
		}

		result, err := parseApiRequest(req)

		assert.NilError(t, err)
		assert.DeepEqual(
			t, result, &eventOperation{
				Subscribe, "mbland@acm.org", uuid.Nil, false,
			},
		)
	})

	t.Run("SuccessfulOneClickUnsubscribe", func(t *testing.T) {
		// The "email" and "uid" are path parameters. "List-Unsubscribe" is
		// parsed from the body.
		const uidStr = "00000000-1111-2222-3333-444444444444"

		req := &apiRequest{
			RawPath: UnsubscribePrefix + "/mbland@acm.org/" + uidStr,
			Params: map[string]string{
				"email": "mbland@acm.org", "uid": uidStr,
			},
			Method:      http.MethodPost,
			ContentType: "application/x-www-form-urlencoded",
			Body:        "List-Unsubscribe=One-Click",
		}

		result, err := parseApiRequest(req)

		assert.NilError(t, err)
		assert.DeepEqual(t, result, &eventOperation{
			Unsubscribe, "mbland@acm.org", uuid.MustParse(uidStr), true,
		})
	})
}

func TestCheckForOnlyOneAddress(t *testing.T) {
	t.Run("MissingAddress", func(t *testing.T) {
		err := checkForOnlyOneAddress("From", []string{})

		assert.Error(t, err, "missing From address")
	})
	t.Run("MoreThanOneAddress", func(t *testing.T) {
		froms := []string{"mbland@acm.org", "foobar@example.com"}
		err := checkForOnlyOneAddress("From", froms)

		expected := "more than one From address: " + strings.Join(froms, ",")
		assert.Error(t, err, expected)
	})
}

func TestCheckMailAddresses(t *testing.T) {
	emptyAddrs := []string{}
	froms := []string{"mbland@acm.org"}
	unsubscribeAddr := "unsubscribe@mike-bland.com"
	tos := []string{unsubscribeAddr}

	t.Run("MissingFromAddress", func(t *testing.T) {
		err := checkMailAddresses(emptyAddrs, tos, unsubscribeAddr)
		assert.Error(t, err, "missing From address")
	})

	t.Run("MissingToAddress", func(t *testing.T) {
		err := checkMailAddresses(froms, emptyAddrs, unsubscribeAddr)
		assert.Error(t, err, "missing To address")
	})

	t.Run("InvalidToAddress", func(t *testing.T) {
		toAddr := "foobar@mike-bland.com"

		err := checkMailAddresses(froms, []string{toAddr}, unsubscribeAddr)

		expected := fmt.Sprintf(
			"not addressed to %s: %s", unsubscribeAddr, toAddr,
		)
		assert.Error(t, err, expected)
	})

	t.Run("Success", func(t *testing.T) {
		assert.NilError(t, checkMailAddresses(froms, tos, unsubscribeAddr))
	})
}

func TestParseEmailSubject(t *testing.T) {
	email := "mbland@acm.org"
	uidStr := "00000000-1111-2222-3333-444444444444"
	uid := uuid.MustParse(uidStr)

	t.Run("EmptyString", func(t *testing.T) {
		result, err := parseEmailSubject("")

		assert.DeepEqual(t, nilSubject, result)
		assert.Error(t, err, `subject not in "<email> <uid>" format: ""`)
	})

	t.Run("WhitespaceOnly", func(t *testing.T) {
		result, err := parseEmailSubject(" ")

		assert.DeepEqual(t, nilSubject, result)
		assert.Error(t, err, `subject not in "<email> <uid>" format: " "`)
	})

	t.Run("BlankEmail", func(t *testing.T) {
		subject := " " + uidStr
		result, err := parseEmailSubject(subject)

		assert.DeepEqual(t, nilSubject, result)
		assert.Error(
			t, err, `subject not in "<email> <uid>" format: "`+subject+`"`,
		)
	})

	t.Run("BlankUid", func(t *testing.T) {
		subject := email + " "
		result, err := parseEmailSubject(subject)

		assert.DeepEqual(t, nilSubject, result)
		assert.Error(
			t, err, `subject not in "<email> <uid>" format: "`+subject+`"`,
		)
	})

	t.Run("InvalidEmail", func(t *testing.T) {
		subject := "mbland+acm.org " + uidStr
		result, err := parseEmailSubject(subject)

		assert.DeepEqual(t, nilSubject, result)
		assert.ErrorContains(t, err, "invalid email address: mbland+acm.org")
	})

	t.Run("InvalidUid", func(t *testing.T) {
		subject := email + " 0123456789"
		result, err := parseEmailSubject(subject)

		assert.DeepEqual(t, nilSubject, result)
		assert.ErrorContains(t, err, "invalid uid: 0123456789")
	})

	t.Run("Success", func(t *testing.T) {
		result, err := parseEmailSubject(email + " " + uidStr)

		assert.NilError(t, err)
		assert.DeepEqual(t, &parsedSubject{email, uid}, result)
	})
}

func TestParseMailtoEvent(t *testing.T) {
	froms := []string{"mbland@acm.org"}
	unsubscribeAddr := "unsubscribe@mike-bland.com"
	tos := []string{unsubscribeAddr}
	email := "mbland@acm.org"
	uidStr := "00000000-1111-2222-3333-444444444444"
	uid := uuid.MustParse(uidStr)
	subject := email + " " + uidStr

	t.Run("MissingFromAddress", func(t *testing.T) {
		result, err := parseMailtoEvent(
			&mailtoEvent{To: tos, Subject: subject}, unsubscribeAddr,
		)

		assert.Assert(t, is.Nil(result))
		assert.Error(t, err, "missing From address")
	})

	t.Run("EmptySubject", func(t *testing.T) {
		result, err := parseMailtoEvent(
			&mailtoEvent{From: froms, To: tos}, unsubscribeAddr,
		)
		assert.Assert(t, is.Nil(result))
		assert.Error(t, err, `subject not in "<email> <uid>" format: ""`)
	})

	t.Run("Success", func(t *testing.T) {
		result, err := parseMailtoEvent(
			&mailtoEvent{From: froms, To: tos, Subject: subject},
			unsubscribeAddr,
		)

		assert.NilError(t, err)
		assert.DeepEqual(
			t, &eventOperation{Unsubscribe, email, uid, true}, result,
		)
	})
}
