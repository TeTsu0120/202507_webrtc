#!/bin/sh

# DockerコンテナのデフォルトゲートウェイがホストIP（bridgeネットワーク前提）
HOST_IP=$(ip route | awk '/default/ { print $3 }')
export HOST_IP

echo "[INFO] Detected HOST_IP: $HOST_IP"

# アプリケーション実行（例：Goバイナリ）
exec /app/main
