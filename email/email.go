package email

import (
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
)

const ExampleMessageJson = `  {
    "From": "Foo Bar <foobar@example.com>",
    "Subject": "Test object",
    "TextBody": "Hello, World!",
    "TextFooter": "Unsubscribe: ` + UnsubscribeUrlTemplate + `",
    "HtmlBody": "<!DOCTYPE html><html><head></head><body>Hello, World!<br/>",
    "HtmlFooter": "<a href='` + UnsubscribeUrlTemplate +
	`'>Unsubscribe</a></body></html>"
  }`

var ExampleMessage *Message = MustParseMessageFromJson(
	strings.NewReader(ExampleMessageJson),
)

var ExampleRecipient *Recipient = func() (r *Recipient) {
	r = &Recipient{
		Email: "subscriber@foo.com",
		Uid:   uuid.MustParse("00000000-1111-2222-3333-444444444444"),
	}
	r.SetUnsubscribeInfo("unsubscribe@bar.com", "https://bar.com/email/")
	return
}()

func EmitPreviewMessageFromJson(input io.Reader, output io.Writer) error {
	if mt, err := NewMessageTemplateFromJson(input); err != nil {
		return err
	} else if err = mt.EmitMessage(output, ExampleRecipient); err != nil {
		return fmt.Errorf("failed to emit preview message: %w", err)
	}
	return nil
}
