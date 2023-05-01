package email

import (
	"bytes"

	"github.com/google/uuid"
)

const UnsubscribeUrlTemplate = "{{UnsubscribeUrl}}"

var unsubscribeUrlTemplate = []byte(UnsubscribeUrlTemplate)

type Subscriber struct {
	Email       string
	Uid         uuid.UUID
	unsubUrl    []byte
	unsubHeader []byte
}

func (sub *Subscriber) SetUnsubscribeInfo(email, baseUrl string) {
	uid := sub.Uid.String()
	b := &bytes.Buffer{}

	b.Reset()
	b.WriteString(baseUrl)
	b.WriteString(sub.Email)
	b.WriteString("/")
	b.WriteString(uid)
	sub.unsubUrl = make([]byte, b.Len())
	copy(sub.unsubUrl, b.Bytes())

	b.Reset()
	b.WriteString("List-Unsubscribe: <mailto:")
	b.WriteString(email)
	b.WriteString("?subject=")
	b.WriteString(sub.Email)
	b.WriteString("%20")
	b.WriteString(uid)
	b.WriteString(">, <")
	b.Write(sub.unsubUrl)
	b.WriteString(">\r\n")
	sub.unsubHeader = make([]byte, b.Len())
	copy(sub.unsubHeader, b.Bytes())
}

func (sub *Subscriber) FillInUnsubscribeUrl(msg []byte) []byte {
	return bytes.Replace(msg, unsubscribeUrlTemplate, sub.unsubUrl, 1)
}
