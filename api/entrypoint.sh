#!/bin/sh
set -e

if [ -n "$DATABASE_URL" ]; then
  echo "[entrypoint] waiting for database..."
  READY=0
  for i in $(seq 1 60); do
    if psql "$DATABASE_URL" -qc 'select 1' >/dev/null 2>&1; then
      READY=1
      break
    fi
    printf '.'
    sleep 1
  done
  echo
  if [ "$READY" -ne 1 ]; then
    echo "[entrypoint] database not reachable after timeout" >&2
    exit 1
  fi

  echo "[entrypoint] running migrations..."
  /app/migrate.sh up
else
  echo "[entrypoint] DATABASE_URL not set; skipping wait + migrations"
fi

echo "[entrypoint] starting server..."
exec /app/server
