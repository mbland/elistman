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
  LOCAL=1
fi

EXIT_CODE=0

printf_with_highlight() {
  local style_code="$1"
  local prefix="$2"
  shift 2

  if [[ -t 1 ]]; then
    printf "%b%s" "${style_code}" "${prefix}"
    printf "$@"
    printf "%b" '\033[0m'
  else
    printf "*** ${prefix}: "
    printf "$@"
  fi
}

printf_info() {
  printf_with_highlight '\033[1;36m' "INFO: " "$@"
}

printf_pass() {
  printf_with_highlight '\033[1;32m' "PASSED: " "$@"
}

printf_fail() {
  printf_with_highlight '\033[1;31m' "FAILED: " "$@"
}

expect_status_from_endpoint() {
  local status="$1"
  local method="$2"
  local endpoint="${BASE_URL}/${3}"

  printf_info "Expect %s from: %s %s\n" "$status" "$method" "$endpoint"

  local curl_cmd=("curl" "-isS" "-X" "$method" "$endpoint")
  local response="$("${curl_cmd[@]}")"

  printf "%s\n\n%s\n\n" "${curl_cmd[*]}" "${response}"

  local response_status=""

  if [[ "$response" =~ HTTP/[^\ ]+\ ([1-5][0-9][0-9]) ]]; then
    response_status="${BASH_REMATCH[1]}"

    if [[ "$response_status" == "$status" ]]; then
      printf_pass "%s\n\n" "$status"
    else
      printf_fail "Expected %s, actual %s:\n" "$status" "$response_status"
      ((EXIT_CODE+=1))
    fi

  else
    printf_fail "Couldn't determine response status from:\n"
    ((EXIT_CODE+=1))
  fi
}

expect_status_from_endpoint 303 POST \
  'subscribe/mbland%40acm.org'
expect_status_from_endpoint 303 GET \
  'verify/mbland%40acm.org/00000000-1111-2222-3333-444444444444'

not_found_status=404
if [[ -n "$LOCAL" ]]; then
  not_found_status=403
fi

expect_status_from_endpoint "$not_found_status" POST \
  'foobar/mbland%40acm.org'
expect_status_from_endpoint 303 POST \
  'subscribe/foo%20bar'
expect_status_from_endpoint 400 GET \
  'verify/foobar/00000000-1111-2222-3333-444444444444'
expect_status_from_endpoint 400 GET \
  'unsubscribe/mbland%40acm.org/bad-uid'

if [[ "$EXIT_CODE" -eq 0 ]]; then
  printf_pass "All smoke tests passed!\n"
else
  printf_fail "Some expectations failed; see above output for details.\n"
fi
exit "$EXIT_CODE"
