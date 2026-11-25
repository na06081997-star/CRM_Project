#!/usr/bin/env bash
# Runs the test suites the way CI does. Starts a throw-away PostgreSQL for the
# integration tests when TEST_DATABASE_URL is not set.
#
#   scripts/test.sh            # backend (unit + integration) and frontend
#   scripts/test.sh backend    # Go only
#   scripts/test.sh frontend   # React only
#   scripts/test.sh watch      # frontend tests in watch mode (red → green loop)
set -euo pipefail
cd "$(dirname "$0")/.."

what="${1:-all}"
PG_CONTAINER=crm-test-pg
PG_PORT=55433

ensure_db() {
  if [ -n "${TEST_DATABASE_URL:-}" ]; then return; fi
  if ! docker ps --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
    echo "▶ starting $PG_CONTAINER (postgres:16-alpine on :$PG_PORT)"
    docker run -d --rm --name "$PG_CONTAINER" -e POSTGRES_PASSWORD=pg -e POSTGRES_DB=crm_test \
      -p "$PG_PORT:5432" postgres:16-alpine >/dev/null
    for _ in $(seq 1 30); do
      docker exec "$PG_CONTAINER" pg_isready -U postgres >/dev/null 2>&1 && break
      sleep 1
    done
  fi
  export TEST_DATABASE_URL="postgres://postgres:pg@localhost:$PG_PORT/crm_test?sslmode=disable"
}

backend() {
  ensure_db
  (cd backend && test -z "$(gofmt -l .)" || { gofmt -l .; echo "gofmt needed"; exit 1; })
  (cd backend && go vet -tags integration ./... && go test -count=1 ./... && go test -tags integration -count=1 -p 1 ./...)
}

frontend() {
  (cd frontend && npm run lint && npm test)
}

case "$what" in
  backend) backend ;;
  frontend) frontend ;;
  watch) cd frontend && npm run test:watch ;;
  all) backend && frontend ;;
  *) echo "usage: $0 [all|backend|frontend|watch]"; exit 2 ;;
esac
