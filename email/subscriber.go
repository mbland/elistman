package email

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const UnsubscribeUrlTemplate = "{{UnsubscribeUrl}}"

type Subscriber struct {
	Email       string
	Uid         uuid.UUID
	UnsubMailto string
	UnsubUrl    string
	UnsubHeader string
}

func (sub *Subscriber) SetUnsubscribeInfo(email, baseUrl string) {
	uid := sub.Uid.String()
	sub.UnsubMailto = "mailto:" + email + "?subject=" + sub.Email + "%20" + uid
	sub.UnsubUrl = baseUrl + sub.Email + "/" + uid

	const hdrFmt = "List-Unsubscribe: <%s>, <%s>"
	sub.UnsubHeader = fmt.Sprintf(hdrFmt, sub.UnsubMailto, sub.UnsubUrl)
}

func (sub *Subscriber) AddUnsubscribeUrl(msg string) string {
	return strings.Replace(msg, UnsubscribeUrlTemplate, sub.UnsubUrl, 1)
}
