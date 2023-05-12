package ops

import (
	"strings"

	"github.com/google/uuid"
)

const (
	ApiPrefixSubscribe   = "/subscribe"
	ApiPrefixVerify      = "/verify/"
	ApiPrefixUnsubscribe = "/unsubscribe/"
)

func VerifyUrl(apiBaseUrl, emailAddr string, uid uuid.UUID) string {
	return makeApiUrl(apiBaseUrl, ApiPrefixVerify, emailAddr, uid)
}

func UnsubscribeUrl(apiBaseUrl, emailAddr string, uid uuid.UUID) string {
	return makeApiUrl(apiBaseUrl, ApiPrefixUnsubscribe, emailAddr, uid)
}

func UnsubscribeMailto(unsubEmail, emailAddr string, uid uuid.UUID) string {
	sb := strings.Builder{}
	sb.WriteString("mailto:")
	sb.WriteString(unsubEmail)
	sb.WriteString("?subject=")
	sb.WriteString(emailAddr)
	sb.WriteString("%20")
	sb.WriteString(uid.String())
	return sb.String()
}

func makeApiUrl(baseUrl, opPrefix, emailAddr string, uid uuid.UUID) string {
	sb := strings.Builder{}
	sb.WriteString(baseUrl)
	sb.WriteString(opPrefix)
	sb.WriteString(emailAddr)
	sb.WriteString("/")
	sb.WriteString(uid.String())
	return sb.String()
}
