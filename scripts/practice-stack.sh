#!/usr/bin/env bash
set -euo pipefail

# 独立项目名让 Compose 为练习创建单独的容器、网络和数据卷。
cd "$(dirname "$0")/.."
exec docker compose --project-name study-go-practice \
  -f labs/stack/compose.yaml -f labs/practice/compose.yaml "$@"
