#!/bin/bash

tool_path() {
  command -v "$1" || command -v "${1}.exe" || command -v "${1}.cmd"
}

tool_version() {
  local tool="$1"
  local version_flag="$2"

  if [[ "$tool" =~ .cmd$ && "$OSTYPE" != "msys" ]]; then
    cmd.exe /C "${tool##*/}" "$version_flag"
  else
    "$tool" "$version_flag"
  fi
}

find_tool() {
  local tool="$1"
  local version_flag="$2"

  tool="$(tool_path "$tool")"

  if [[ -z "$tool" ]]; then
    return 1
  fi

  local version="$(tool_version "$tool" "$version_flag")"
  printf "Found: %s\n       %s\n" "$tool" "${version%%$'\n'*}"
}

check_for_tool() {
  local tool_label="Required"
  if [[ "$1" == "--optional" ]]; then
    tool_label="Optional"
    shift
  fi

  local tool="$1"
  local version_flag="$2"
  local msg="$3"

  if ! find_tool "$tool" "$version_flag"; then
    printf "%s tool not found: %s\n       %s\n" "$tool_label" "$tool" "$msg" >&2
    [[ "$tool_label" == "Optional" ]] || ((EXIT_CODE+=1))
  fi
}

EXIT_CODE=0

check_for_tool go version \
  "See https://go.dev/dl/ or https://github.com/syndbg/goenv"
check_for_tool aws --version \
  "See https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
check_for_tool sam --version \
  "See https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html"

check_for_tool --optional docker --version \
  "See https://docs.docker.com/get-docker/"
check_for_tool --optional curl --version \
  "See https://curl.se/download.html or https://winget.run/pkg/cURL/cURL"

if [[ $EXIT_CODE -ne 0 ]]; then
  printf "\n%s\n" \
    "Some required tools are missing; see the output above for guidance." >&2
fi
exit $EXIT_CODE
