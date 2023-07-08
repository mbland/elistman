package email

import (
	"bytes"
	"io"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/mbland/elistman/ops"
)

const UnsubscribeUrlTemplate = "{{UnsubscribeUrl}}"

var unsubscribeUrlTemplate = []byte(UnsubscribeUrlTemplate)

type Recipient struct {
	Email        string
	Uid          uuid.UUID
	unsubFormUrl []byte
	unsubApiUrl  []byte
	unsubHeader  []byte
}

func (sub *Recipient) SetUnsubscribeInfo(email, formUrl, apiBaseUrl string) {
	sub.unsubFormUrl = unsubscribeFormUrl(formUrl, sub.Email, sub.Uid)
	sub.unsubApiUrl = []byte(ops.UnsubscribeUrl(apiBaseUrl, sub.Email, sub.Uid))

	sb := &strings.Builder{}
	sb.WriteString("List-Unsubscribe: <")
	sb.WriteString(ops.UnsubscribeMailto(email, sub.Email, sub.Uid))
	sb.WriteString(">, <")
	sb.Write(sub.unsubApiUrl)
	sb.WriteString(">\r\n")
	sub.unsubHeader = []byte(sb.String())
}

func unsubscribeFormUrl(baseFormUrl, email string, uid uuid.UUID) []byte {
	sb := &strings.Builder{}
	sb.WriteString(baseFormUrl)
	sb.WriteString("?email=")
	sb.WriteString(url.QueryEscape(email))
	sb.WriteString("&uid=")
	sb.WriteString(uid.String())
	return []byte(sb.String())
}

var listUnsubscribePost = []byte(
	"List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n",
)

func (sub *Recipient) EmitUnsubscribeHeaders(w io.Writer) (err error) {
	// If unsubHeader is empty, this is a verification message. No need for the
	// unsubscribe info if the subscriber isn't yet verified.
	if len(sub.unsubHeader) == 0 {
		return
	} else if _, err = w.Write(sub.unsubHeader); err != nil {
		return
	}
	_, err = w.Write(listUnsubscribePost)
	return
}

func (sub *Recipient) FillInUnsubscribeUrl(msg []byte) []byte {
	return bytes.Replace(msg, unsubscribeUrlTemplate, sub.unsubFormUrl, 1)
}
