#!/usr/bin/env bash
set -euo pipefail

if [[ $# != 2 ]]; then
  printf '用法：bash scripts/check-exercise.sh {stock-lookup|order-history} {go|java}\n' >&2
  exit 2
fi

exercise=$1
language=$2
case "$exercise" in
  stock-lookup) check_class=StockLookupCheck ;;
  order-history) check_class=OrderHistoryCheck ;;
  *) printf '未知练习：%s\n' "$exercise" >&2; exit 2 ;;
esac

cd "$(dirname "$0")/../exercises"
case "$language" in
  go) exec go test -v "./$exercise/go" ;;
  java)
    # 临时编译目录由脚本创建，退出时删除生成的 .class 文件。
    classes="$(mktemp -d)"
    trap 'rm -r -- "$classes"' EXIT
    javac -encoding UTF-8 -d "$classes" "$exercise"/java/*.java
    java -ea -cp "$classes" "$check_class"
    ;;
  *) printf '请选择语言 go 或 java。\n' >&2; exit 2 ;;
esac
