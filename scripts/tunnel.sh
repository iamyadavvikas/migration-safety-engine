#!/usr/bin/env bash
set -euo pipefail

# Migration Safety Engine — public tunnel
# Usage: ./scripts/tunnel.sh [cloudflare|serveo] [start|stop|status]
# Requires: cloudflared (for Cloudflare mode) or ssh (for Serveo fallback)

MODE="${1:-cloudflare}"
ACTION="${2:-status}"
LOCAL_PORT="${ENGINE_ADDR:-8080}"
LOCAL_PORT="${LOCAL_PORT#*:}"
PIDFILE="/tmp/mse-tunnel-${MODE}.pid"
LOGFILE="/tmp/mse-tunnel-${MODE}.log"

cloudflare_start() {
  if command -v cloudflared &>/dev/null; then
    echo "Starting Cloudflare Tunnel → localhost:${LOCAL_PORT}"
    cloudflared tunnel --no-autoupdate run \
      > "$LOGFILE" 2>&1 &
    PID=$!
    echo "$PID" > "$PIDFILE"
    echo "Cloudflare Tunnel started (PID $PID)"
    echo "Go to Cloudflare Zero Trust dashboard → Tunnels to get your URL"
  else
    echo "cloudflared not found. Install: brew install cloudflared"
    echo "Then: cloudflared tunnel login"
    exit 1
  fi
}

serveo_start() {
  SUBDOMAIN="${SERVEO_SUBDOMAIN:-mse-demo}"
  REMOTE_PORT=80
  echo "Starting Serveo tunnel: ${SUBDOMAIN}.serveousercontent.com → localhost:${LOCAL_PORT}"
  ssh -o StrictHostKeyChecking=no \
      -o ServerAliveInterval=30 \
      -o ServerAliveCountMax=3 \
      -R "${SUBDOMAIN}:${REMOTE_PORT}:localhost:${LOCAL_PORT}" \
      -N serveo.net \
      >> "$LOGFILE" 2>&1 &
  PID=$!
  echo "$PID" > "$PIDFILE"
  echo "Serveo tunnel started (PID $PID)"
  sleep 3
  if kill -0 "$PID" 2>/dev/null; then
    echo "https://${SUBDOMAIN}.serveousercontent.com"
  else
    echo "failed. Check $LOGFILE"
    rm -f "$PIDFILE"
    exit 1
  fi
}

stop() {
  if [ ! -f "$PIDFILE" ]; then
    echo "No tunnel running for mode: ${MODE}"
    return
  fi
  PID=$(cat "$PIDFILE")
  kill "$PID" 2>/dev/null || true
  rm -f "$PIDFILE"
  echo "Tunnel stopped"
}

status() {
  if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
    echo "Tunnel ${MODE} is running (PID $(cat "$PIDFILE"))"
    return 0
  else
    echo "Tunnel ${MODE} is not running"
    return 1
  fi
}

case "$ACTION" in
  start)
    case "$MODE" in
      cloudflare) cloudflare_start ;;
      serveo)     serveo_start ;;
      *)          echo "mode: cloudflare|serveo"; exit 1 ;;
    esac
    ;;
  stop)  stop ;;
  status) status ;;
  *)     echo "Usage: $0 [cloudflare|serveo] [start|stop|status]"; exit 1 ;;
esac
