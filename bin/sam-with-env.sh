#!/bin/bash

ENV_FILE="$1"
shift

if [[ -z "$ENV_FILE" ]]; then
  printf 'Usage: %s [deployment environment variables file] [sam args...]\n' \
    "$0" >&2
  exit 1
elif [[ "$#" -eq 0 ]]; then
  printf "No arguments for 'sam' command given.\n" >&2
  exit 1
fi
. "$ENV_FILE" || exit 1

printf "${SENDER_NAME:?}" >/dev/null
PARAMETER_OVERRIDES=(
  "ApiDomainName=${API_DOMAIN_NAME:?}"
  "ApiMappingKey=${API_MAPPING_KEY:?}"
  "EmailDomainName=${EMAIL_DOMAIN_NAME:?}"
  "EmailSiteTitle=${EMAIL_SITE_TITLE:?}"
  "SenderName=${SENDER_NAME// /\ }"
  "SubscribersTableName=${SUBSCRIBERS_TABLE_NAME:?}"
  "InvalidRequestPath=${INVALID_REQUEST_PATH:?}"
  "AlreadySubscribedPath=${ALREADY_SUBSCRIBED_PATH:?}"
  "VerifyLinkSentPath=${VERIFY_LINK_SENT_PATH:?}"
  "SubscribedPath=${SUBSCRIBED_PATH:?}"
  "NotSubscribedPath=${NOT_SUBSCRIBED_PATH:?}"
  "UnsubscribedPath=${UNSUBSCRIBED_PATH:?}"
)

exec sam "${@}" --parameter-overrides "${PARAMETER_OVERRIDES[*]}"
