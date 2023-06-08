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

  # goenv will install a shim into ${GOENV_ROOT}/shims for every tool installed
  # into ${GOPATH}/bin via `go install`. If the current Go version doesn't have
  # a tool, but another version does, the shim emits a "command not found"
  # message to stderr. So we check for "command not found" to ensure the script
  # will reinstall the tool for the current Go version.
  local version="$(tool_version "$tool" "$version_flag" 2>&1)"

  if [[ "$version" =~ \ command\ not\ found ]]; then
    return 1
  fi
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

install_tool() {
  local tool="$1"
  local version_flag="$2"
  local msg="$3"
  local install_tool="$4"
  shift 4
  local install_cmd=("$(tool_path "$install_tool")" "${@}")

  if find_tool "$tool" "$version_flag"; then
    return
  elif [[ -z "${install_cmd[0]}" ]]; then
    printf "Could not install '%s' because '%s' not found\n" \
      "$tool" "$install_tool" >&2
    ((EXIT_CODE+=1))
  elif ! "${install_cmd[@]}"; then
    printf "Failed to install '%s' via: %s\n  %s\n" \
      "$tool" "${install_cmd[*]}" "$msg" >&2
    ((EXIT_CODE+=1))
  else
    tool="$(tool_path "$tool")"
    printf "Installed: %s\n       %s\n" "$tool" "$("$tool" "$version_flag")"
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

install_tool staticcheck --version "See https://staticcheck.io" \
  go install honnef.co/go/tools/cmd/staticcheck@latest

if [[ $EXIT_CODE -ne 0 ]]; then
  printf "\n%s\n" \
    "Some required tools are missing; see the output above for guidance." >&2
fi
exit $EXIT_CODE
