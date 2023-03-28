package handler

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"gotest.tools/assert"
)

func TestParseError(t *testing.T) {
	err := &ParseError{
		Type:     SubscribeOp,
		Endpoint: "/subscribe/mbland acm.org",
		Message:  "invalid email parameter",
	}

	t.Run("StringIncludesOptypeMessageAndEndpoint", func(t *testing.T) {
		assert.Equal(
			t,
			"Subscribe: invalid email parameter: /subscribe/mbland acm.org",
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
			t, err, &ParseError{UndefinedOp, "/foobar/baz", "unknown endpoint"},
		)
	})
}

// This series of parseEmail tests serves to test the underlying
// parseEmailAddress and parsePathParam functions.
func TestParseEmail(t *testing.T) {
	t.Run("ParamMissing", func(t *testing.T) {
		pi := &pathInfo{VerifyOp, "/foobar", map[string]string{}}
		result, err := pi.parseEmail()

		assert.Equal(t, "", result)
		assert.DeepEqual(
			t, err, &ParseError{VerifyOp, "/foobar", "missing email parameter"},
		)
	})

	t.Run("ParamInvalid", func(t *testing.T) {
		pi := &pathInfo{
			VerifyOp, "/foobar/bazquux", map[string]string{"email": "bazquux"},
		}
		result, err := pi.parseEmail()

		assert.Equal(t, "", result)
		assert.DeepEqual(t, err, &ParseError{
			VerifyOp,
			"/foobar/bazquux",
			"invalid email parameter: bazquux: mail: missing '@' or angle-addr",
		})
	})

	t.Run("ParamValid", func(t *testing.T) {
		pi := &pathInfo{
			SubscribeOp,
			"/subscribe/mbland@acm.org",
			map[string]string{"email": "mbland@acm.org"},
		}
		result, err := pi.parseEmail()

		assert.NilError(t, err)
		assert.Equal(t, "mbland@acm.org", result)
	})
}

func TestParseUid(t *testing.T) {
	t.Run("IgnoreSubscribeOp", func(t *testing.T) {
		pi := &pathInfo{SubscribeOp, SubscribePrefix, map[string]string{}}
		result, err := pi.parseUid()

		assert.NilError(t, err)
		assert.Equal(t, uuid.Nil, result)
	})

	t.Run("ParamValid", func(t *testing.T) {
		expected, err := uuid.Parse("00000000-1111-2222-3333-444444444444")
		assert.NilError(t, err)

		pi := &pathInfo{
			VerifyOp,
			"/verify/mbland@acm.org/" + expected.String(),
			map[string]string{"uid": expected.String()},
		}
		result, err := pi.parseUid()

		assert.NilError(t, err)
		assert.Equal(t, expected, result)
	})
}

func TestParseApiEvent(t *testing.T) {
	t.Run("Unknown", func(t *testing.T) {
		result, err := parseApiEvent("/foobar", map[string]string{})

		assert.Assert(t, result == nil)
		assert.Assert(t, errors.Is(err, &ParseError{Type: UndefinedOp}))
		assert.ErrorContains(t, err, "unknown endpoint: /foobar")
	})

	t.Run("InvalidEmail", func(t *testing.T) {
		result, err := parseApiEvent(
			SubscribePrefix+"foobar", map[string]string{"email": "foobar"},
		)

		assert.Assert(t, result == nil)
		assert.Assert(t, errors.Is(err, &ParseError{Type: SubscribeOp}))
		assert.ErrorContains(t, err, "invalid email parameter: foobar")
	})

	t.Run("InvalidUid", func(t *testing.T) {
		result, err := parseApiEvent(
			VerifyPrefix+"mbland@acm.org/0123456789",
			map[string]string{"email": "mbland@acm.org", "uid": "0123456789"},
		)

		assert.Assert(t, result == nil)
		assert.Assert(t, errors.Is(err, &ParseError{Type: VerifyOp}))
		assert.ErrorContains(t, err, "invalid uid parameter: 0123456789")
	})

	t.Run("Success", func(t *testing.T) {
		uuidStr := "00000000-1111-2222-3333-444444444444"
		result, err := parseApiEvent(
			UnsubscribePrefix+"/mbland@acm.org/"+uuidStr,
			map[string]string{"email": "mbland@acm.org", "uid": uuidStr},
		)

		assert.NilError(t, err)
		assert.DeepEqual(t, result, &eventOperation{
			UnsubscribeOp, "mbland@acm.org", uuid.MustParse(uuidStr),
		})
	})
}
