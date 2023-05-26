//go:build small_tests || all_tests

package handler

import (
	"errors"
	"testing"

	"github.com/mbland/elistman/testutils"
	"github.com/mbland/elistman/types"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestUndefinedEnvVarsErrorFormat(t *testing.T) {
	assert.ErrorContains(
		t,
		&UndefinedEnvVarsError{UndefinedVars: []string{"FOO", "BAR", "BAZ"}},
		"undefined environment variables: FOO, BAR, BAZ",
	)
}

func TestReportUndefinedEnviromentVariables(t *testing.T) {
	opts, err := GetOptions(func(string) string { return "" })

	assert.Assert(t, is.Nil(opts))

	undefErr := &UndefinedEnvVarsError{}
	expectedErr := &UndefinedEnvVarsError{
		UndefinedVars: []string{
			"API_DOMAIN_NAME",
			"API_MAPPING_KEY",
			"EMAIL_DOMAIN_NAME",
			"EMAIL_SITE_TITLE",
			"SENDER_NAME",
			"SENDER_USER_NAME",
			"UNSUBSCRIBE_USER_NAME",
			"SUBSCRIBERS_TABLE_NAME",
			"CONFIGURATION_SET",
			"MAX_BULK_SEND_CAPACITY",
			"INVALID_REQUEST_PATH",
			"ALREADY_SUBSCRIBED_PATH",
			"VERIFY_LINK_SENT_PATH",
			"SUBSCRIBED_PATH",
			"NOT_SUBSCRIBED_PATH",
			"UNSUBSCRIBED_PATH",
		},
	}
	assert.Assert(t, errors.As(err, &undefErr))
	assert.DeepEqual(t, expectedErr, undefErr)
}

func testEnv() (env map[string]string, getenv func(string) string) {
	env = map[string]string{
		"API_DOMAIN_NAME":         "api.mike-bland.com",
		"API_MAPPING_KEY":         "email",
		"EMAIL_DOMAIN_NAME":       "mike-bland.com",
		"EMAIL_SITE_TITLE":        "Mike Bland's blog",
		"SENDER_NAME":             "Mike Bland",
		"SENDER_USER_NAME":        "no-reply",
		"UNSUBSCRIBE_USER_NAME":   "unsubscribe",
		"SUBSCRIBERS_TABLE_NAME":  "subscribers",
		"CONFIGURATION_SET":       "config-set",
		"MAX_BULK_SEND_CAPACITY":  "0.8",
		"INVALID_REQUEST_PATH":    "/invalid",
		"ALREADY_SUBSCRIBED_PATH": "/already-subscribed",
		"VERIFY_LINK_SENT_PATH":   "/verify",
		"SUBSCRIBED_PATH":         "/subscribed",
		"NOT_SUBSCRIBED_PATH":     "/not-subscribed",
		"UNSUBSCRIBED_PATH":       "/unsubscribed",
	}
	getenv = func(varname string) string {
		return env[varname]
	}
	return
}

func TestAllRequiredEnvironmentVariablesDefined(t *testing.T) {
	_, getenv := testEnv()

	opts, err := GetOptions(getenv)

	assert.NilError(t, err)
	expectedCapacity, _ := types.NewCapacity(0.8)
	assert.DeepEqual(
		t,
		opts,
		&Options{
			ApiDomainName:        "api.mike-bland.com",
			ApiMappingKey:        "email",
			EmailDomainName:      "mike-bland.com",
			EmailSiteTitle:       "Mike Bland's blog",
			SenderName:           "Mike Bland",
			SenderUserName:       "no-reply",
			UnsubscribeUserName:  "unsubscribe",
			SubscribersTableName: "subscribers",
			ConfigurationSet:     "config-set",
			MaxBulkSendCapacity:  expectedCapacity,

			// Note that GetOptions will remove a leading '/' character from the
			// path value.
			RedirectPaths: RedirectPaths{
				Invalid:           "invalid",
				AlreadySubscribed: "already-subscribed",
				VerifyLinkSent:    "verify",
				Subscribed:        "subscribed",
				NotSubscribed:     "not-subscribed",
				Unsubscribed:      "unsubscribed",
			},
		},
	)
}

func TestOptionsAssignCapacityAddsError(t *testing.T) {
	// Note that the success and undefined cases are covered by the tests above.

	t.Run("IfOutsideRange", func(t *testing.T) {
		env, getenv := testEnv()
		env["MAX_BULK_SEND_CAPACITY"] = "1.1"

		opts, err := GetOptions(getenv)

		assert.Assert(t, is.Nil(opts))
		assert.Assert(t, testutils.ErrorIs(err, types.ErrInvalidCapacity))
		assert.ErrorContains(t, err, "invalid MAX_BULK_SEND_CAPACITY: ")
	})

	t.Run("IfWrongDataType", func(t *testing.T) {
		env, getenv := testEnv()
		env["MAX_BULK_SEND_CAPACITY"] = "foobar"

		opts, err := GetOptions(getenv)

		assert.Assert(t, is.Nil(opts))
		assert.Assert(t, testutils.ErrorIsNot(err, types.ErrInvalidCapacity))
		assert.ErrorContains(t, err, "invalid MAX_BULK_SEND_CAPACITY: ")
		assert.ErrorContains(t, err, "strconv.ParseFloat")
	})
}

func TestOptionsReturnsMultipleWrappedErrors(t *testing.T) {
	env, getenv := testEnv()
	delete(env, "SENDER_NAME")
	env["MAX_BULK_SEND_CAPACITY"] = "1.1"

	opts, err := GetOptions(getenv)

	undefErr := &UndefinedEnvVarsError{}
	expectedErr := &UndefinedEnvVarsError{
		UndefinedVars: []string{"SENDER_NAME"},
	}
	assert.Assert(t, is.Nil(opts))
	assert.Assert(t, errors.As(err, &undefErr))
	assert.DeepEqual(t, expectedErr, undefErr)
	assert.Assert(t, testutils.ErrorIs(err, types.ErrInvalidCapacity))
}
