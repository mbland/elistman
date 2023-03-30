package handler

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestUnknownEventOperationType(t *testing.T) {
	unknownOp := UndefinedOp - 1
	assert.Equal(t, "Unknown", unknownOp.String())
}

func TestParseError(t *testing.T) {
	err := &ParseError{
		Type:    SubscribeOp,
		Message: "invalid email parameter: mbland acm.org",
	}

	t.Run("StringIncludesOptypeMessageAndEndpoint", func(t *testing.T) {
		assert.Equal(
			t,
			"Subscribe: invalid email parameter: mbland acm.org",
			err.Error(),
		)
	})

	t.Run("IsTrue", func(t *testing.T) {
		assert.Assert(t, err.Is(&ParseError{}), "empty ParseError")
		assert.Assert(t, err.Is(&ParseError{Type: SubscribeOp}), "Type match")
	})

	t.Run("IsFalse", func(t *testing.T) {
		stringErr := fmt.Errorf(
			"Subscribe: invalid email parameter: /subscribe/mbland acm.org",
		)

		assert.Assert(t, !err.Is(stringErr), "string")
		assert.Assert(t, !err.Is(&ParseError{Type: VerifyOp}), "Type mismatch")
	})
}

func TestParseOperationType(t *testing.T) {
	t.Run("Subscribe", func(t *testing.T) {
		result, err := parseOperationType(SubscribePrefix + "/foobar")

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
		assert.DeepEqual(
			t, err, &ParseError{UndefinedOp, "unknown endpoint: /foobar/baz"},
		)
	})
}

// This series of parseEmail tests serves to test the underlying
// parseEmailAddress and parseParam functions.
func TestParseEmail(t *testing.T) {
	t.Run("ParamMissing", func(t *testing.T) {
		pi := &opInfo{VerifyOp, map[string]string{}}
		result, err := pi.parseEmail()

		assert.Equal(t, "", result)
		assert.DeepEqual(
			t, err, &ParseError{VerifyOp, "missing email parameter"},
		)
	})

	t.Run("ParamInvalid", func(t *testing.T) {
		pi := &opInfo{VerifyOp, map[string]string{"email": "bazquux"}}
		result, err := pi.parseEmail()

		assert.Equal(t, "", result)
		assert.DeepEqual(t, err, &ParseError{
			VerifyOp,
			"invalid email parameter: bazquux: mail: missing '@' or angle-addr",
		})
	})

	t.Run("ParamValid", func(t *testing.T) {
		pi := &opInfo{SubscribeOp, map[string]string{"email": "mbland@acm.org"}}
		result, err := pi.parseEmail()

		assert.NilError(t, err)
		assert.Equal(t, "mbland@acm.org", result)
	})
}

func TestParseUid(t *testing.T) {
	t.Run("IgnoreSubscribeOp", func(t *testing.T) {
		pi := &opInfo{SubscribeOp, map[string]string{}}
		result, err := pi.parseUid()

		assert.NilError(t, err)
		assert.Equal(t, uuid.Nil, result)
	})

	t.Run("ParamValid", func(t *testing.T) {
		expected, err := uuid.Parse("00000000-1111-2222-3333-444444444444")
		assert.NilError(t, err)

		pi := &opInfo{VerifyOp, map[string]string{"uid": expected.String()}}
		result, err := pi.parseUid()

		assert.NilError(t, err)
		assert.Equal(t, expected, result)
	})
}

func TestParseApiEvent(t *testing.T) {
	t.Run("Unknown", func(t *testing.T) {
		result, err := parseApiRequest(&apiRequest{
			RawPath: "/foobar", Params: map[string]string{},
		})

		assert.Assert(t, is.Nil(result))
		assert.Assert(t, errors.Is(err, &ParseError{Type: UndefinedOp}))
		assert.ErrorContains(t, err, "unknown endpoint: /foobar")
	})

	t.Run("InvalidEmail", func(t *testing.T) {
		result, err := parseApiRequest(&apiRequest{
			RawPath: SubscribePrefix + "foobar",
			Params:  map[string]string{"email": "foobar"},
		})

		assert.Assert(t, is.Nil(result))
		assert.Assert(t, errors.Is(err, &ParseError{Type: SubscribeOp}))
		assert.ErrorContains(t, err, "invalid email parameter: foobar")
	})

	t.Run("InvalidUid", func(t *testing.T) {
		result, err := parseApiRequest(&apiRequest{
			RawPath: VerifyPrefix + "mbland@acm.org/0123456789",
			Params: map[string]string{
				"email": "mbland@acm.org", "uid": "0123456789",
			},
		})

		assert.Assert(t, is.Nil(result))
		assert.Assert(t, errors.Is(err, &ParseError{Type: VerifyOp}))
		assert.ErrorContains(t, err, "invalid uid parameter: 0123456789")
	})

	t.Run("Success", func(t *testing.T) {
		uidStr := "00000000-1111-2222-3333-444444444444"
		result, err := parseApiRequest(&apiRequest{
			RawPath: UnsubscribePrefix + "/mbland@acm.org/" + uidStr,
			Params: map[string]string{
				"email": "mbland@acm.org", "uid": uidStr,
			},
		})

		assert.NilError(t, err)
		assert.DeepEqual(t, result, &eventOperation{
			UnsubscribeOp, "mbland@acm.org", uuid.MustParse(uidStr),
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
		assert.Error(t, err, "subject not in `<email> <uid>` format: \"\"")
	})

	t.Run("WhitespaceOnly", func(t *testing.T) {
		result, err := parseEmailSubject(" ")

		assert.DeepEqual(t, nilSubject, result)
		assert.Error(t, err, "subject not in `<email> <uid>` format: \" \"")
	})

	t.Run("BlankEmail", func(t *testing.T) {
		subject := " " + uidStr
		result, err := parseEmailSubject(subject)

		assert.DeepEqual(t, nilSubject, result)
		assert.Error(
			t, err, "subject not in `<email> <uid>` format: \""+subject+"\"",
		)
	})

	t.Run("BlankUid", func(t *testing.T) {
		subject := email + " "
		result, err := parseEmailSubject(subject)

		assert.DeepEqual(t, nilSubject, result)
		assert.Error(
			t, err, "subject not in `<email> <uid>` format: \""+subject+"\"",
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
		assert.Error(t, err, "subject not in `<email> <uid>` format: \"\"")
	})

	t.Run("Success", func(t *testing.T) {
		result, err := parseMailtoEvent(
			&mailtoEvent{From: froms, To: tos, Subject: subject},
			unsubscribeAddr,
		)

		assert.NilError(t, err)
		assert.DeepEqual(t, &eventOperation{UnsubscribeOp, email, uid}, result)
	})
}
