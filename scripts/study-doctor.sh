#!/usr/bin/env bash
set -u

usage() {
  printf '用法：bash scripts/study-doctor.sh [--profile basics|web|electron|agent] [--docker]\n'
}

profile=basics
check_docker=0
while (( $# > 0 )); do
  case "$1" in
    --profile)
      if (( $# < 2 )); then
        usage >&2
        exit 2
      fi
      case "$2" in
        basics|web|electron|agent) profile=$2 ;;
        *) usage >&2; exit 2 ;;
      esac
      shift 2
      ;;
    --docker) check_docker=1; shift ;;
    --help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
done

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
  local minimum_minor=$1 output
  output=$(go version 2>&1) || { printf '%s\n' "$output"; return 1; }
  if [[ $output =~ go([0-9]+)\.([0-9]+) ]] &&
    (( BASH_REMATCH[1] > 1 || (BASH_REMATCH[1] == 1 && BASH_REMATCH[2] >= minimum_minor) )); then
    printf '%s\n' "$output"
  else
    printf '需要 Go 1.%s+，当前为 %s\n' "$minimum_minor" "$output"
    return 1
  fi
}

java_runtime_version() {
  local output line version_major=0
  output=$(java -version 2>&1) || { printf '%s\n' "$output"; return 1; }
  # 同时检查运行器和编译器，避免 PATH 中的 java 与 javac 指向不同版本。
  # 按版本行取数字，启动选项提示中的内存大小等数字不参与判断。
  while IFS= read -r line; do
    if [[ $line =~ ^(openjdk|java)[[:space:]]+(version[[:space:]]+)?\"?([0-9]+) ]]; then
      version_major=${BASH_REMATCH[3]}
      break
    fi
  done <<< "$output"
  if (( version_major >= 21 )); then
    printf '%s\n' "$line"
  else
    printf '需要 Java 21+，当前为 %s\n' "$output"
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

python_version() {
  local output
  output=$(python3 --version 2>&1) || { printf '%s\n' "$output"; return 1; }
  if [[ $output =~ Python[[:space:]]+([0-9]+)\.([0-9]+) ]] &&
    (( BASH_REMATCH[1] > 3 || (BASH_REMATCH[1] == 3 && BASH_REMATCH[2] >= 9) )); then
    printf '%s\n' "$output"
  else
    printf '需要 Python 3.9+，当前为 %s\n' "$output"
    return 1
  fi
}

node_version() {
  local minimum_major=$1 minimum_minor=$2 output
  output=$(node --version 2>&1) || { printf '%s\n' "$output"; return 1; }
  if [[ $output =~ ^v([0-9]+)\.([0-9]+)\. ]] &&
    (( BASH_REMATCH[1] > minimum_major ||
      (BASH_REMATCH[1] == minimum_major && BASH_REMATCH[2] >= minimum_minor) )); then
    printf '%s\n' "$output"
  else
    printf '需要 Node.js %s.%s+，当前为 %s\n' "$minimum_major" "$minimum_minor" "$output"
    return 1
  fi
}

check_basics() {
  check Go go_version 22
  check Java java_runtime_version
  check JDK java_version
  check Maven mvn -version
  check Python python_version
  check curl curl --version
}

# 默认对应语法与内存 HTTP 课程，各阶段分别使用仓库声明的最低版本。
printf '检查阶段 %s\n' "$profile"
case "$profile" in
  basics) check_basics ;;
  web)
    check Node.js node_version 18 0
    check npm npm --version
    check Python python_version
    ;;
  electron)
    check_basics
    check Node.js node_version 22 12
    check npm npm --version
    ;;
  agent)
    check Go go_version 26
    check Python python_version
    check curl curl --version
    ;;
esac

if (( check_docker )); then
  check 'Docker Compose' docker compose version
  check 'Docker Engine' docker version --format 'Server {{.Server.Version}}'
fi

exit "$failed"
