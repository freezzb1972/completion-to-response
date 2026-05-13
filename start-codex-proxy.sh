#!/bin/bash
# Codex Proxy — Responses API → Chat Completions → DeepSeek
# Usage: ./start-codex-proxy.sh

DIR="$(cd "$(dirname "$0")" && pwd)"
BIN="$DIR/completion-to-response"
URL="${BACKEND_URL:-https://api.deepseek.com/v1/chat/completions}"
PORT="${PROXY_PORT:-8082}"
DEFAULT_MODEL="${DEFAULT_MODEL:-deepseek-v4-flash}"

# 如果设置了 FORCE_MODEL，强制覆盖客户端模型；否则自动穿透
MODEL_FLAG=""
if [ -n "${FORCE_MODEL:-}" ]; then
    MODEL_FLAG="-model $FORCE_MODEL"
fi

if [ ! -f "$BIN" ]; then
    echo "Binary not found: $BIN"
    echo "Download it from: https://github.com/NoahStepheno/completion-to-response/releases/latest"
    exit 1
fi

echo "=== Codex Proxy ==="
echo "Upstream:  $URL"
echo "Default:   $DEFAULT_MODEL (used when client doesn't specify)"
[ -n "$FORCE_MODEL" ] && echo "Override:  $FORCE_MODEL (always use this)"
echo "Port:      $PORT"
echo ""

exec "$BIN" -url "$URL" -default-model "$DEFAULT_MODEL" $MODEL_FLAG -port "$PORT"
