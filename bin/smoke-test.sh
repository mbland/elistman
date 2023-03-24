#!/bin/bash
#
# Checks that the service is running responding to requests.

BASE_URL="https://api.mike-bland.com/email"

if [[ "$1" == "--local" ]]; then
  if [[ -z "$(ps -ef | grep -- '[s]am local start-api --port 8080')" ]]; then
    printf '%s\n' "Please run 'make run-local' in a separate shell first." >&2
    exit 1
  fi
  BASE_URL="http://127.0.0.1:8080"
fi

set -xe 

echo Expect 303
curl -i -X POST "${BASE_URL}/subscribe/mbland%40acm.org"

echo Expect 303
curl -i "${BASE_URL}/verify/mbland%40acm.org/00000000-1111-2222-3333-444444444444"

echo Expect 404
curl -i -X POST "${BASE_URL}/foobar/mbland%40acm.org"

echo Expect 303
curl -i -X POST "${BASE_URL}/subscribe/foo%20bar"

echo Expect 400
curl -i "${BASE_URL}/verify/foobar/00000000-1111-2222-3333-444444444444"

echo Expect 400
curl -i "${BASE_URL}/unsubscribe/mbland%40acm.org/bad-uid"
