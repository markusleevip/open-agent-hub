#!/usr/bin/env bash
#
# Start Open Agent Hub backend (Console REST API + MCP Gateway, single process, dual servers).
#
# Usage:
#   ./scripts/start-backend.sh            # start in background (default: build then run)
#   ./scripts/start-backend.sh start      # start in background
#   ./scripts/start-backend.sh stop       # stop background service
#   ./scripts/start-backend.sh restart    # restart background service
#   ./scripts/start-backend.sh status     # show background service status
#   ./scripts/start-backend.sh logs       # tail background service logs
#   ./scripts/start-backend.sh run        # foreground go run (dev/debug)
#   ./scripts/start-backend.sh build      # compile binary then run
#   ./scripts/start-backend.sh test       # run go test ./...
#
# Config: the script auto-loads backend/.env if present. All variables have defaults,
# see backend/.env.example.
#
set -euo pipefail

# Locate repo root (script lives under <repo>/scripts/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BACKEND_DIR="${REPO_ROOT}/backend"
RUN_DIR="${BACKEND_DIR}/tmp"
PID_FILE="${RUN_DIR}/open-agent-hub.pid"
LOG_FILE="${RUN_DIR}/open-agent-hub.log"
BIN_FILE="${RUN_DIR}/openagent-server"

cd "${BACKEND_DIR}"

# 1. Load .env if present. Export line by line, ignore comments and blank lines.
if [[ -f .env ]]; then
  echo "==> Loading backend/.env"
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
else
  echo "==> backend/.env not found, using built-in defaults (see .env.example)"
fi

# 2. Validate Go toolchain
if ! command -v go >/dev/null 2>&1; then
  echo "Error: go not found. Please install the Go toolchain (go.mod requires $(grep '^go ' go.mod | awk '{print $2}'))" >&2
  exit 1
fi

# 3. Ensure SQLite data directory exists (DB_DSN defaults to data/openagenthub.db)
DB_TYPE="${DB_TYPE:-sqlite}"
DB_DSN="${DB_DSN:-data/openagenthub.db}"
if [[ "${DB_TYPE}" == "sqlite" ]]; then
  mkdir -p "$(dirname "${DB_DSN}")"
fi
mkdir -p "${RUN_DIR}"

MODE="${1:-start}"
echo "==> Console :${CONSOLE_PORT:-8084}  |  MCP Gateway :${MCP_PORT:-8085}  |  DB:${DB_TYPE}"

is_running() {
  local pid="${1:-}"
  [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1
}

pid_from_file() {
  if [[ -f "${PID_FILE}" ]]; then
    tr -d '[:space:]' < "${PID_FILE}"
  fi
}

wait_for_exit() {
  local pid="$1"
  local timeout="${2:-10}"
  local elapsed=0
  while is_running "${pid}" && [[ "${elapsed}" -lt "${timeout}" ]]; do
    sleep 1
    elapsed=$((elapsed + 1))
  done
}

start_background() {
  local existing_pid
  existing_pid="$(pid_from_file || true)"
  if is_running "${existing_pid}"; then
    echo "==> Backend already running in background, PID=${existing_pid}"
    echo "==> Logs: ${LOG_FILE}"
    return 0
  fi
  if [[ -n "${existing_pid}" ]]; then
    echo "==> Cleaning stale PID file: ${PID_FILE}"
    rm -f "${PID_FILE}"
  fi

  echo "==> go build -o ${BIN_FILE} ./cmd/server/"
  go build -o "${BIN_FILE}" ./cmd/server/

  echo "==> Starting in background: ${BIN_FILE}"
  nohup "${BIN_FILE}" > "${LOG_FILE}" 2>&1 &
  local pid=$!
  echo "${pid}" > "${PID_FILE}"

  sleep 1
  if ! is_running "${pid}"; then
    echo "Error: background service failed to start, recent logs:" >&2
    tail -n 80 "${LOG_FILE}" >&2 || true
    rm -f "${PID_FILE}"
    exit 1
  fi

  echo "==> Backend started, PID=${pid}"
  echo "==> Console: http://localhost:${CONSOLE_PORT:-8084}"
  echo "==> MCP:     http://localhost:${MCP_PORT:-8085}/mcp"
  echo "==> Logs: ${LOG_FILE}"
}

stop_background() {
  local pid
  pid="$(pid_from_file || true)"
  if ! is_running "${pid}"; then
    echo "==> Backend is not running"
    rm -f "${PID_FILE}"
    return 0
  fi

  echo "==> Stopping backend, PID=${pid}"
  kill "${pid}" >/dev/null 2>&1 || true
  wait_for_exit "${pid}" 12
  if is_running "${pid}"; then
    echo "==> Graceful shutdown timed out, force killing PID=${pid}"
    kill -9 "${pid}" >/dev/null 2>&1 || true
  fi
  rm -f "${PID_FILE}"
  echo "==> Stopped"
}

show_status() {
  local pid
  pid="$(pid_from_file || true)"
  if is_running "${pid}"; then
    echo "==> Backend running, PID=${pid}"
    echo "==> Console: http://localhost:${CONSOLE_PORT:-8084}"
    echo "==> MCP:     http://localhost:${MCP_PORT:-8085}/mcp"
    echo "==> Logs: ${LOG_FILE}"
  else
    echo "==> Backend is not running"
    if [[ -f "${PID_FILE}" ]]; then
      echo "==> Stale PID file found: ${PID_FILE}"
    fi
  fi
}

case "${MODE}" in
  start)
    start_background
    ;;
  stop)
    stop_background
    ;;
  restart)
    stop_background
    start_background
    ;;
  status)
    show_status
    ;;
  logs)
    touch "${LOG_FILE}"
    echo "==> tail -f ${LOG_FILE}"
    exec tail -f "${LOG_FILE}"
    ;;
  run)
    echo "==> go run ./cmd/server/"
    exec go run ./cmd/server/
    ;;
  build)
    echo "==> go build -o openagent-bin ./cmd/server/"
    go build -o openagent-bin ./cmd/server/
    echo "==> ./openagent-bin"
    exec ./openagent-bin
    ;;
  test)
    echo "==> go test ./..."
    exec go test ./...
    ;;
  *)
    echo "Unknown mode: ${MODE} (options: start | stop | restart | status | logs | run | build | test)" >&2
    exit 1
    ;;
esac
