#!/bin/bash

DEPLOY_ENV="$1"

if [[ -z "$DEPLOY_ENV" ]]; then
  printf 'Usage: %s [deployment environment variables file]\n' "$0" >&2
  exit 1
elif [[ ! -r "$DEPLOY_ENV" ]]; then
  printf 'Deployment environment variable file missing or not readable: %s\n' \
    "$DEPLOY_ENV" >&2
  exit 1
fi

. "$DEPLOY_ENV"

PARAMETER_OVERRIDES=(
  "ApiDomainName=${API_DOMAIN_NAME}"
  "SenderArn=${SENDER_ARN}"
  "SenderEmailAddress=${SENDER_EMAIL_ADDRESS}"
  "SenderName=${SENDER_NAME// /\ }"
)

exec sam deploy --parameter-overrides "${PARAMETER_OVERRIDES[*]}"
