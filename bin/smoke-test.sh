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
  local description="$1"
  local status="$2"
  local method="$3"
  local endpoint="${BASE_URL}/${4}"
  local content_type="$5"
  local num_shift_args="$(($# < 5 ? $# : 5))"
  local curl_data_flag="-d"  # for application/x-www-form-urlencoded
  local postdata=()

  if [[ "$content_type" == "multipart/form-data" ]]; then
    curl_data_flag="-F"
  fi
  shift "$num_shift_args"

  for arg in "$@"; do
    postdata+=("$curl_data_flag" "$arg")
  done

  printf_info "TEST: %s\nExpect %s from: %s %s\n" \
    "$description" "$status" "$method" "$endpoint"

  local curl_cmd=("curl" "-isS" "-X" "$method" "${postdata[@]}" "$endpoint")
  local response="$("${curl_cmd[@]}")"

  printf "%s\n\n%s\n" "${curl_cmd[*]}" "${response/%$'\n'}"
  local response_status=""

  if [[ "$response" =~ HTTP/[^\ ]+\ ([1-5][0-9][0-9]) ]]; then
    response_status="${BASH_REMATCH[1]}"

    if [[ "$response_status" == "$status" ]]; then
      printf_pass "%s: %s\n\n" "$description" "$status"
    else
      printf_fail "%s: Expected %s, actual %s\n\n" \
        "$description" "$status" "$response_status"
      ((EXIT_CODE+=1))
    fi

  else
    printf_fail "%s: Couldn't determine response status\n\n" "$description"
    ((EXIT_CODE+=1))
  fi
}

printf_info "SUITE: Success\n"
expect_status_from_endpoint \
  'successful subscribe using urlencoded params' \
  303 POST \
  'subscribe' \
  'application/x-www-form-urlencoded' \
  'email=mbland%40acm.org'
expect_status_from_endpoint \
  'successful subscribe using form-data' \
  303 POST \
  'subscribe' \
  'multipart/form-data' \
  'email=mbland%40acm.org'
expect_status_from_endpoint \
  "successful verify" \
  303 GET \
  'verify/mbland%40acm.org/00000000-1111-2222-3333-444444444444'
expect_status_from_endpoint \
  "unsubscribe" \
  303 GET \
  "unsubscribe/mbland%40acm.org/00000000-1111-2222-3333-444444444444"
expect_status_from_endpoint \
  'one-click unsubscribe using urlencoded params' \
  200 POST \
  'unsubscribe/mbland%40acm.org/00000000-1111-2222-3333-444444444444' \
  'application/x-www-form-urlencoded' \
  'List-Unsubscribe=One-Click'
expect_status_from_endpoint \
  'one-click unsubscribe using form-data' \
  200 POST \
  "unsubscribe/mbland%40acm.org/00000000-1111-2222-3333-444444444444" \
  'multipart/form-data' \
  'List-Unsubscribe=One-Click'

printf_info "SUITE: Not found (403 locally, 404 in prod)\n"

not_found_status=404
if [[ -n "$LOCAL" ]]; then
  not_found_status=403
fi

expect_status_from_endpoint \
  "invalid endpoint not found" \
  "$not_found_status" POST \
  'foobar/mbland%40acm.org'

printf_info "%s\n" \
  "SUITE: Redirect if missing or invalid email address for /subscribe"

missing_address_status=303
if [[ -n "$LOCAL" ]]; then
  missing_address_status=403
fi

expect_status_from_endpoint \
  "missing email address (403 locally, 303 in prod)" \
  "$missing_address_status" POST \
  'subscribe'
expect_status_from_endpoint \
  "invalid email address" \
  303 POST \
  'subscribe'
  'application/x-www-form-urlencoded' \
  'email=foo%20bar'

printf_info "SUITE: All other missing or invalid parameters return 400\n"
expect_status_from_endpoint \
  "invalid email address for /verify" \
  400 GET \
  'verify/foobar/00000000-1111-2222-3333-444444444444'
expect_status_from_endpoint \
  "invalid UID for /unsubscribe" \
  400 GET \
  'unsubscribe/mbland%40acm.org/bad-uid'

if [[ "$EXIT_CODE" -eq 0 ]]; then
  printf_pass "All smoke tests passed!\n"
else
  printf_fail "Some expectations failed; see above output for details.\n"
fi
exit "$EXIT_CODE"
