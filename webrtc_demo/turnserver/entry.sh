#!/bin/sh

# ホストIPを取得（bridgeネットワーク想定）
HOST_IP=$(ip route | awk '/default/ { print $3 }')
echo "[entrypoint] Detected HOST_IP=$HOST_IP"

# 設定ファイルに挿入（テンプレートから生成）
envsubst < /etc/turnserver/turnserver.conf.template > /etc/turnserver/turnserver.conf

exec turnserver -c /etc/turnserver/turnserver.conf --no-cli
