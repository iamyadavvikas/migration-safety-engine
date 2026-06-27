#!/usr/bin/env bash
set -euo pipefail

# Migration Safety Engine — public tunnel
# Usage: ./scripts/tunnel.sh [start|stop|status]
# Requires: ssh (for Serveo), or a fallback tunnel工具

SUBDOMAIN="mse-demo"
REMOTE_PORT=80
LOCAL_PORT="${ENGINE_ADDR:-8080}"
LOCAL_PORT="${LOCAL_PORT#*:}"  # strip host prefix if "host:port"
PIDFILE="/tmp/mse-tunnel.pid"
LOGFILE="/tmp/mse-tunnel.log"

start() {
  if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
    echo "Tunnel already running (PID $(cat "$PIDFILE"))"
    exit 1
  fi

  echo "Starting tunnel: ${SUBDOMAIN}.serveousercontent.com → localhost:${LOCAL_PORT}"

  # Serveo (primary)
  ssh -o StrictHostKeyChecking=no \
      -o ServerAliveInterval=30 \
      -o ServerAliveCountMax=3 \
      -R "${SUBDOMAIN}:${REMOTE_PORT}:localhost:${LOCAL_PORT}" \
      -N serveo.net \
      >> "$LOGFILE" 2>&1 &

  PID=$!
  echo "$PID" > "$PIDFILE"
  echo "Tunnel started (PID $PID)"

  # Verify after 3s
  sleep 3
  if kill -0 "$PID" 2>/dev/null; then
    echo "Tunnel is running: https://${SUBDOMAIN}.serveousercontent.com"
  else
    echo "Tunnel failed to start. Check $LOGFILE"
    rm -f "$PIDFILE"
    exit 1
  fi
}

stop() {
  if [ ! -f "$PIDFILE" ]; then
    echo "No tunnel PID file found"
    return
  fi

  PID=$(cat "$PIDFILE")
  if kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    echo "Tunnel stopped (PID $PID)"
  else
    echo "Tunnel was not running"
  fi
  rm -f "$PIDFILE"
}

status() {
  if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
    echo "Tunnel is running (PID $(cat "$PIDFILE"))"
    echo "URL: https://${SUBDOMAIN}.serveousercontent.com"
  else
    echo "Tunnel is not running"
  fi
}

case "${1:-status}" in
  start) start ;;
  stop)  stop  ;;
  status) status ;;
  *)     echo "Usage: $0 [start|stop|status]"; exit 1 ;;
esac
