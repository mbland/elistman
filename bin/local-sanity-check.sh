#!/bin/bash
#
# Checks that the service is running locally and responding to requests.

if [[ -z "$(ps -ef | grep -- '[s]am local start-api --port 8080')" ]]; then
  printf '%s\n' "Please run 'make run-local' in a separate shell first." >&2
  exit 1
fi

set -xe 
curl -d "email=mbland@acm.org" -i http://127.0.0.1:8080/new
curl -i http://127.0.0.1:8080/validate
