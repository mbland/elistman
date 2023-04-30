package email

import (
	"io"
)

type Builder struct {
	buf io.Writer
}

func (b *Builder) buildListMessage(
	msg *MessageTemplate, sub *Subscriber,
) error {
	w := &Writer{buf: b.buf}

	w.WriteLine(msg.From)
	w.WriteLine("To: " + sub.Email)
	w.WriteLine(msg.Subject)
	w.WriteLine(sub.UnsubHeader)
	w.WriteLine("List-Unsubscribe-Post: List-Unsubscribe=One-Click")
	w.WriteLine("MIME-Version: 1.0")

	if len(msg.HtmlBody) == 0 {
		msg.EmitTextOnly(w, sub)
	} else {
		msg.EmitMultipart(w, sub)
	}
	return w.err
}
