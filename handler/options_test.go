package handler

import (
	"testing"

	"gotest.tools/assert"
)

func TestUndefinedEnvVarsErrorFormat(t *testing.T) {
	assert.ErrorContains(
		t,
		&UndefinedEnvVarsError{UndefinedVars: []string{"FOO", "BAR", "BAZ"}},
		"undefined environment variables: FOO, BAR, BAZ",
	)
}

func TestReportUndefinedEnviromentVariables(t *testing.T) {
	_, err := GetOptions(func(string) string { return "" })

	assert.DeepEqual(
		t,
		err,
		&UndefinedEnvVarsError{
			UndefinedVars: []string{
				"API_DOMAIN_NAME",
				"API_MAPPING_KEY",
				"EMAIL_DOMAIN_NAME",
				"EMAIL_SITE_TITLE",
				"SENDER_NAME",
				"SENDER_USER_NAME",
				"UNSUBSCRIBE_USER_NAME",
				"SUBSCRIBERS_TABLE_NAME",
				"INVALID_REQUEST_PATH",
				"ALREADY_SUBSCRIBED_PATH",
				"VERIFY_LINK_SENT_PATH",
				"SUBSCRIBED_PATH",
				"NOT_SUBSCRIBED_PATH",
				"UNSUBSCRIBED_PATH",
			},
		},
	)
}

func TestAllRequiredEnvironmentVariablesDefined(t *testing.T) {
	env := map[string]string{
		"API_DOMAIN_NAME":         "api.mike-bland.com",
		"API_MAPPING_KEY":         "email",
		"EMAIL_DOMAIN_NAME":       "mike-bland.com",
		"EMAIL_SITE_TITLE":        "Mike Bland's blog",
		"SENDER_NAME":             "Mike Bland",
		"SENDER_USER_NAME":        "no-reply",
		"UNSUBSCRIBE_USER_NAME":   "unsubscribe",
		"SUBSCRIBERS_TABLE_NAME":  "subscribers",
		"INVALID_REQUEST_PATH":    "/invalid",
		"ALREADY_SUBSCRIBED_PATH": "/already-subscribed",
		"VERIFY_LINK_SENT_PATH":   "/verify",
		"SUBSCRIBED_PATH":         "/subscribed",
		"NOT_SUBSCRIBED_PATH":     "/not-subscribed",
		"UNSUBSCRIBED_PATH":       "/unsubscribed",
	}
	opts, err := GetOptions(func(varname string) string {
		return env[varname]
	})

	assert.NilError(t, err)
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
