package handler

import (
	"fmt"
	"strings"
)

type RedirectUrls struct {
	Invalid           string
	AlreadySubscribed string
	VerifyLinkSent    string
	Subscribed        string
	NotSubscribed     string
	Unsubscribed      string
}

type Options struct {
	ApiDomainName        string
	ApiMappingKey        string
	EmailDomainName      string
	SenderName           string
	SubscribersTableName string

	RedirectUrls RedirectUrls
}

func GetOptions(getenv func(string) string) (*Options, error) {
	env := environment{getenv: getenv}
	return env.options()
}

type environment struct {
	getenv      func(string) string
	missingVars []string
}

func (env *environment) options() (*Options, error) {
	opts := Options{}
	env.assign(&opts.ApiDomainName, "API_DOMAIN_NAME")
	env.assign(&opts.ApiMappingKey, "API_MAPPING_KEY")
	env.assign(&opts.EmailDomainName, "EMAIL_DOMAIN_NAME")
	env.assign(&opts.SenderName, "SENDER_NAME")
	env.assign(&opts.SubscribersTableName, "SUBSCRIBERS_TABLE_NAME")

	redirects := &opts.RedirectUrls
	env.assign(&redirects.Invalid, "INVALID_REQUEST_URL")
	env.assign(&redirects.AlreadySubscribed, "ALREADY_SUBSCRIBED_URL")
	env.assign(&redirects.VerifyLinkSent, "VERIFY_LINK_SENT_URL")
	env.assign(&redirects.Subscribed, "SUBSCRIBED_URL")
	env.assign(&redirects.NotSubscribed, "NOT_SUBSCRIBED_URL")
	env.assign(&redirects.Unsubscribed, "UNSUBSCRIBED_URL")

	if len(env.missingVars) != 0 {
		return nil, fmt.Errorf(
			"undefined environment variables:\n  %s",
			strings.Join(env.missingVars, "\n  "),
		)
	}
	return &opts, nil
}

func (env *environment) assign(opt *string, varname string) {
	if value := env.getenv(varname); value == "" {
		env.missingVars = append(env.missingVars, varname)
	} else {
		*opt = value
	}
}
