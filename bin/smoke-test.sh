#!/bin/bash
#
# Checks that the service is running responding to requests.

ENV_FILE="$1"
shift

if [[ -z "$ENV_FILE" ]]; then
  printf 'Usage: %s [deployment environment variables file]\n' "$0" >&2
  exit 1
fi
. "$ENV_FILE" || exit 1

BASE_URL="https://${API_DOMAIN_NAME:?}/${API_MAPPING_KEY:?}"

if [[ "$1" == '--local' ]]; then
  if [[ -z "$(ps -ef | grep -- '[s]am local start-api --port 8080')" ]]; then
    printf '%s\n' 'Please run 'make run-local' in a separate shell first.' >&2
    exit 1
  fi
  BASE_URL='http://127.0.0.1:8080'
  LOCAL=1
fi

EMAIL_DOMAIN_NAME="${EMAIL_DOMAIN_NAME:?}"
INVALID_REQUEST_PATH="${EMAIL_DOMAIN_NAME}/${INVALID_REQUEST_PATH:?}"
INVALID_REQUEST_PATH="https://${INVALID_REQUEST_PATH//\/\///}"
ALREADY_SUBSCRIBED_PATH="${EMAIL_DOMAIN_NAME}/${ALREADY_SUBSCRIBED_PATH:?}"
ALREADY_SUBSCRIBED_PATH="https://${ALREADY_SUBSCRIBED_PATH//\/\///}"
VERIFY_LINK_SENT_PATH="${EMAIL_DOMAIN_NAME}/${VERIFY_LINK_SENT_PATH:?}"
VERIFY_LINK_SENT_PATH="https://${VERIFY_LINK_SENT_PATH//\/\///}"
SUBSCRIBED_PATH="${EMAIL_DOMAIN_NAME}/${SUBSCRIBED_PATH:?}"
SUBSCRIBED_PATH="https://${SUBSCRIBED_PATH//\/\///}"
NOT_SUBSCRIBED_PATH="${EMAIL_DOMAIN_NAME}/${NOT_SUBSCRIBED_PATH:?}"
NOT_SUBSCRIBED_PATH="https://${NOT_SUBSCRIBED_PATH//\/\///}"
UNSUBSCRIBED_PATH="${EMAIL_DOMAIN_NAME}/${UNSUBSCRIBED_PATH:?}"
UNSUBSCRIBED_PATH="https://${UNSUBSCRIBED_PATH//\/\///}"

TEST_CASES=()
FAILED_CASES=()

printf_with_highlight() {
  local style_code="$1"
  local prefix="$2"
  shift 2

  if [[ -t 1 || -n "$SMOKE_TEST_USE_COLOR" ]]; then
    printf '%b%s' "${style_code}" "${prefix}"
    printf "$@"
    printf '%b' '\033[0m'
  else
    printf "*** ${prefix}: "
    printf "$@"
  fi
}

printf_info() {
  printf_with_highlight '\033[1;36m' 'INFO: ' "$@"
}

printf_pass() {
  printf_with_highlight '\033[1;32m' 'PASSED: ' "$@"
}

printf_fail() {
  printf_with_highlight '\033[1;31m' 'FAILED: ' "$@"
}

register_failure() {
  printf_fail "$@"
  FAILED_CASES+=("$(printf "$@")")
}

expect_status_from_endpoint() {
  local description="$((${#TEST_CASES[@]} + 1)) — $1"
  local method="$2"
  local endpoint="${BASE_URL}/${3}"
  local status="$4"
  local location="$5"
  local content_type="$6"
  local num_shift_args="$(($# < 6 ? $# : 6))"
  local postdata=()

  # for application/x-www-form-urlencoded
  local curl_data_flag='--data-urlencode'  

  if [[ "$content_type" == 'multipart/form-data' ]]; then
    curl_data_flag='-F'
  fi
  shift "$num_shift_args"

  for arg in "$@"; do
    postdata+=("$curl_data_flag" "$arg")
  done

  TEST_CASES+=("$description")
  printf_info 'TEST: %s\nExpect %s from: %s %s\n' \
    "$description" "$status" "$method" "$endpoint"

  local curl_cmd=('curl' '-isS' '-X' "$method" "${postdata[@]}" "$endpoint")
  local response="$("${curl_cmd[@]}")"

  printf '\n%s\n\n%s\n\n' "${curl_cmd[*]}" "${response%$'\r\n\r'}"

  local passing_results=()
  local failing_results=()

  check_response_for 'status' 'HTTP/[^\ ]+\ ([1-5][0-9][0-9])' "$status" \
    "$response" "Couldn't determine response status"

  if [[ -n "$location" ]]; then
    check_response_for 'Location' '[Ll]ocation:\ ([^[:space:]]+)' "$location" \
      "$response" "No 'Location:' in response"
  fi

  local result_op="printf_pass"
  if [[ "${#failing_results[@]}" -ne 0 ]]; then
      result_op="register_failure"
  fi
  results="$(printf '    %s\n' "${passing_results[@]}" "${failing_results[@]}")"
  "$result_op" '%s:\n%s\n\n' "$description" "$results"
}

check_response_for() {
  local name="$1"
  local pattern="$2"
  local expected="$3"
  local response="$4"
  local failure_msg="$5"
  local actual=""

  if [[ "$response" =~ $pattern ]]; then
    actual="${BASH_REMATCH[1]}"
  else
    failing_results+=("$(printf "$result_var_name" '%s' "$failure_msg")")
    return 1
  fi

  if [[ "$expected" != "$actual" ]]; then
    failing_results+=(
      "$(printf 'Expected: %s\n    Actual:   %s' "$expected" "$actual")"
    )
    return 1
  fi
  passing_results+=("$(printf '%s: %s' "$name" "$actual")")
}

printf_info 'SUITE: Not found (reported as 403 Forbidden)\n'

not_found_status=403

expect_status_from_endpoint 'invalid endpoint not found' \
  POST 'foobar/mbland%40acm.org' \
  "$not_found_status"

if [[ -n "$LOCAL" ]]; then
    expect_status_from_endpoint '/subscribe with trailing component not found' \
      POST 'subscribe/foobar' \
      "$not_found_status"

    printf_info '%s\n' \
      'SUITE: Redirect if missing or invalid email address for /subscribe'

    expect_status_from_endpoint 'missing email address' \
      POST 'subscribe' \
      303 "$INVALID_REQUEST_PATH" \
      'application/x-www-form-urlencoded' ''

    expect_status_from_endpoint 'invalid email address' \
      POST 'subscribe' \
      303 "$INVALID_REQUEST_PATH" \
      'application/x-www-form-urlencoded' 'email=foo bar'
else
    printf_info '%s\n' \
      'SUITE: /subscribe protected by AWS WAF CAPTCHA'

    expect_status_from_endpoint '/subscribe protected by AWS WAF CAPTCHA' \
      POST 'subscribe' \
      405 "" \
      'application/x-www-form-urlencoded' 'email=smoke-test@example.com'
fi

printf_info 'SUITE: All other missing or invalid parameters return 400\n'

expect_status_from_endpoint 'invalid email address for /verify' \
  GET 'verify/foobar/00000000-1111-2222-3333-444444444444' \
  400 ""

expect_status_from_endpoint 'invalid UID for /unsubscribe' \
  GET 'unsubscribe/mbland%40acm.org/bad-uid' \
  400 ""

if [[ "${#FAILED_CASES[@]}" -eq 0 ]]; then
  printf_pass 'All %d smoke tests passed!\n' "${#TEST_CASES[@]}"
else
  printf_fail '%d/%d tests failed; see above output for details.\n' \
    "${#FAILED_CASES[@]}" "${#TEST_CASES[@]}"
  printf_fail '    %s\n' '' "${FAILED_CASES[@]//    /        }"
fi
exit "${#FAILED_CASES[@]}"
