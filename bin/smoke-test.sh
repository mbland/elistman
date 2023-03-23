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
curl -i -X POST "${BASE_URL}/subscribe/mbland%40acm.org"
curl -i "${BASE_URL}/verify/mbland%40acm.org/00000000-1111-2222-3333-444444444444"
