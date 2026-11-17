#!/usr/bin/env bash
set -u

if [[ $# -gt 1 || (${1:-} != "" && ${1:-} != --docker) ]]; then
  printf '用法：bash scripts/study-doctor.sh [--docker]\n' >&2
  exit 2
fi

failed=0

check() {
  local label=$1 output
  shift
  if output=$("$@" 2>&1); then
    printf 'OK   %-15s %s\n' "$label" "${output%%$'\n'*}"
  else
    printf 'FAIL %-15s %s\n' "$label" "${output%%$'\n'*}" >&2
    failed=1
  fi
}

go_version() {
  local output
  output=$(go version) || return
  if [[ $output =~ go([0-9]+)\.([0-9]+) ]] &&
    (( BASH_REMATCH[1] > 1 || (BASH_REMATCH[1] == 1 && BASH_REMATCH[2] >= 22) )); then
    printf '%s\n' "$output"
  else
    printf '需要 Go 1.22+，当前为 %s\n' "$output"
    return 1
  fi
}

java_version() {
  local output
  output=$(javac -version 2>&1) || { printf '%s\n' "$output"; return 1; }
  if [[ $output =~ javac[[:space:]]+([0-9]+) ]] && (( BASH_REMATCH[1] >= 21 )); then
    printf '%s\n' "$output"
  else
    printf '需要 JDK 21+，当前为 %s\n' "$output"
    return 1
  fi
}

check Go go_version
check JDK java_version
check Maven mvn -version
check Python python3 --version
check curl curl --version
check 'Docker Compose' docker compose version
if [[ ${1:-} == --docker ]]; then
  check 'Docker Engine' docker version --format 'Server {{.Server.Version}}'
fi

exit "$failed"
