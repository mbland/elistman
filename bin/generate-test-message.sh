#!/bin/bash
#
# Generates a test email that can be piped into `elistman send`.

ENV_FILE="$1"
shift

if [[ -z "$ENV_FILE" ]]; then
  printf 'Usage: %s [deployment environment variables file]\n' "$0" >&2
  exit 1
fi
. "$ENV_FILE" || exit 1

cat <<EOF
{
  "From": "${SENDER_NAME:?} <${SENDER_USER_NAME:?}@${EMAIL_DOMAIN_NAME:?}>",
  "Subject": "Test message",
  "TextBody": "Hello, World!",
  "TextFooter": "Unsubscribe: {{UnsubscribeUrl}}",
  "HtmlBody": "<!DOCTYPE html><html><head></head><body>Hello, World!<br/>",
  "HtmlFooter": "<a href='{{UnsubscribeUrl}}'>Unsubscribe</a></body></html>"
}
EOF
