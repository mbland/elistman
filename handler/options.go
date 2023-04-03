package handler

import (
	"strings"
)

type RedirectPaths struct {
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
	EmailSiteTitle       string
	SenderName           string
	SubscribersTableName string

	RedirectPaths RedirectPaths
}

type UndefinedEnvVarsError struct {
	UndefinedVars []string
}

func (e *UndefinedEnvVarsError) Error() string {
	return "undefined environment variables: " +
		strings.Join(e.UndefinedVars, ", ")
}

func GetOptions(getenv func(string) string) (*Options, error) {
	env := environment{getenv: getenv}
	return env.options()
}

type environment struct {
	getenv        func(string) string
	undefinedVars []string
}

func (env *environment) options() (*Options, error) {
	opts := Options{}
	env.assign(&opts.ApiDomainName, "API_DOMAIN_NAME")
	env.assign(&opts.ApiMappingKey, "API_MAPPING_KEY")
	env.assign(&opts.EmailDomainName, "EMAIL_DOMAIN_NAME")
	env.assign(&opts.EmailSiteTitle, "EMAIL_SITE_TITLE")
	env.assign(&opts.SenderName, "SENDER_NAME")
	env.assign(&opts.SubscribersTableName, "SUBSCRIBERS_TABLE_NAME")

	redirects := &opts.RedirectPaths
	env.assignRedirect(&redirects.Invalid, "INVALID_REQUEST_PATH")
	env.assignRedirect(&redirects.AlreadySubscribed, "ALREADY_SUBSCRIBED_PATH")
	env.assignRedirect(&redirects.VerifyLinkSent, "VERIFY_LINK_SENT_PATH")
	env.assignRedirect(&redirects.Subscribed, "SUBSCRIBED_PATH")
	env.assignRedirect(&redirects.NotSubscribed, "NOT_SUBSCRIBED_PATH")
	env.assignRedirect(&redirects.Unsubscribed, "UNSUBSCRIBED_PATH")

	if len(env.undefinedVars) != 0 {
		return nil, &UndefinedEnvVarsError{UndefinedVars: env.undefinedVars}
	}
	return &opts, nil
}

func (env *environment) assign(opt *string, varname string) {
	if value := env.getenv(varname); value == "" {
		env.undefinedVars = append(env.undefinedVars, varname)
	} else {
		*opt = value
	}
}

func (env *environment) assignRedirect(opt *string, varname string) {
	env.assign(opt, varname)
	*opt, _ = strings.CutPrefix(*opt, "/")
}
