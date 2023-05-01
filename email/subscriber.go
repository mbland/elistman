package email

import (
	"strings"

	"github.com/google/uuid"
)

const UnsubscribeUrlTemplate = "{{UnsubscribeUrl}}"

type Subscriber struct {
	Email       string
	Uid         uuid.UUID
	unsubMailto string
	unsubUrl    string
	unsubHeader string
}

func (sub *Subscriber) SetUnsubscribeInfo(email, baseUrl string) {
	uid := sub.Uid.String()
	sb := &strings.Builder{}

	sb.WriteString("mailto:")
	sb.WriteString(email)
	sb.WriteString("?subject=")
	sb.WriteString(sub.Email)
	sb.WriteString("%20")
	sb.WriteString(uid)
	sub.unsubMailto = sb.String()

	sb.Reset()
	sb.WriteString(baseUrl)
	sb.WriteString(sub.Email)
	sb.WriteString("/")
	sb.WriteString(uid)
	sub.unsubUrl = sb.String()

	sb.Reset()
	sb.WriteString("List-Unsubscribe: <")
	sb.WriteString(sub.unsubMailto)
	sb.WriteString(">, <")
	sb.WriteString(sub.unsubUrl)
	sb.WriteString(">")
	sub.unsubHeader = sb.String()
}

func (sub *Subscriber) FillInUnsubscribeUrl(msg string) string {
	return strings.Replace(msg, UnsubscribeUrlTemplate, sub.unsubUrl, 1)
}
