#!/bin/bash

is_installed() {
  local tool_path="$(command -v "$1")"

  [[ -n "$tool_path" ]] && printf "Found: %s\n" "$tool_path"
}

check_for_tool() {
  local tool_label="Required"
  if [[ "$1" == "--optional" ]]; then
    tool_label="Optional"
    shift
  fi

  local tool="$1"
  local msg="$2"

  if ! is_installed "$tool"; then
    printf "%s tool not found: '%s': %s\n" "$tool_label" "$tool" "$msg" >&2
    [[ "$tool_label" == "Optional" ]] || ((EXIT_CODE+=1))
  fi
}

install_tool() {
  local tool="$1"
  shift
  local msg="$1"
  shift
  local install_cmd=("${@}")

  if ! (is_installed "$tool" || "${install_cmd[@]}"); then
    printf "Installation failed: %s\n  %s\n" "${install_cmd[*]}" "$msg" >&2
    ((EXIT_CODE+=1))
  fi
}

EXIT_CODE=0

check_for_tool aws \
  "See https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
check_for_tool sam \
  "See https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html"
check_for_tool go "See https://go.dev/dl/ or https://github.com/syndbg/goenv"

check_for_tool --optional docker "See https://docs.docker.com/get-docker/"

install_tool staticcheck "See https://staticcheck.io" \
  go install honnef.co/go/tools/cmd/staticcheck@latest

if [[ $EXIT_CODE -ne 0 ]]; then
  printf "\n%s\n" \
    "Some required tools are missing; see the output above for guidance." >&2
fi
exit $EXIT_CODE
