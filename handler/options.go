package handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mbland/elistman/types"
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
	SenderUserName       string
	UnsubscribeUserName  string
	SubscribersTableName string
	ConfigurationSet     string
	MaxBulkSendCapacity  types.Capacity

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
	env := environment{getenv: getenv, errors: make([]error, 0)}
	return env.options()
}

type environment struct {
	getenv        func(string) string
	undefinedVars []string
	errors        []error
}

func (env *environment) options() (*Options, error) {
	opts := Options{}
	env.assign(&opts.ApiDomainName, "API_DOMAIN_NAME")
	env.assign(&opts.ApiMappingKey, "API_MAPPING_KEY")
	env.assign(&opts.EmailDomainName, "EMAIL_DOMAIN_NAME")
	env.assign(&opts.EmailSiteTitle, "EMAIL_SITE_TITLE")
	env.assign(&opts.SenderName, "SENDER_NAME")
	env.assign(&opts.SenderUserName, "SENDER_USER_NAME")
	env.assign(&opts.UnsubscribeUserName, "UNSUBSCRIBE_USER_NAME")
	env.assign(&opts.SubscribersTableName, "SUBSCRIBERS_TABLE_NAME")
	env.assign(&opts.ConfigurationSet, "CONFIGURATION_SET")
	env.assignCapacity(&opts.MaxBulkSendCapacity, "MAX_BULK_SEND_CAPACITY")

	redirects := &opts.RedirectPaths
	env.assignRedirect(&redirects.Invalid, "INVALID_REQUEST_PATH")
	env.assignRedirect(&redirects.AlreadySubscribed, "ALREADY_SUBSCRIBED_PATH")
	env.assignRedirect(&redirects.VerifyLinkSent, "VERIFY_LINK_SENT_PATH")
	env.assignRedirect(&redirects.Subscribed, "SUBSCRIBED_PATH")
	env.assignRedirect(&redirects.NotSubscribed, "NOT_SUBSCRIBED_PATH")
	env.assignRedirect(&redirects.Unsubscribed, "UNSUBSCRIBED_PATH")

	if len(env.undefinedVars) != 0 {
		undefErr := &UndefinedEnvVarsError{UndefinedVars: env.undefinedVars}
		env.errors = append(env.errors, undefErr)
	}
	if err := errors.Join(env.errors...); err != nil {
		return nil, err
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

func (env *environment) assignCapacity(opt *types.Capacity, varname string) {
	var capStr string
	var capRaw float64
	var err error

	env.assign(&capStr, varname)
	addErr := func(err error) {
		const errFmt = "invalid %s: %w"
		env.errors = append(env.errors, fmt.Errorf(errFmt, varname, err))
	}

	if len(capStr) == 0 {
		return
	} else if capRaw, err = strconv.ParseFloat(capStr, 64); err != nil {
		addErr(err)
	} else if *opt, err = types.NewCapacity(capRaw); err != nil {
		addErr(err)
	}
}

func (env *environment) assignRedirect(opt *string, varname string) {
	env.assign(opt, varname)
	*opt, _ = strings.CutPrefix(*opt, "/")
}
