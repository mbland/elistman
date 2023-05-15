package email

const ExampleMessageJson = `  {
    "From": "Foo Bar <foobar@example.com>",
    "Subject": "Test object",
    "TextBody": "Hello, World!",
    "TextFooter": "Unsubscribe: ` + UnsubscribeUrlTemplate + `",
    "HtmlBody": "<!DOCTYPE html><html><head></head><body>Hello, World!<br/>",
    "HtmlFooter": "<a href='` + UnsubscribeUrlTemplate +
	`'>Unsubscribe</a></body></html>"
  }`
