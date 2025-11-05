#!/bin/sh
set -e

if [ -z "$DATABASE_URL" ]; then
  echo "[migrate] DATABASE_URL not set" >&2
  exit 1
fi

CMD="up"
if [ -n "$1" ]; then
  CMD="$1"
fi

BASE="/app/migrations"
if [ ! -f "$BASE/001_init.sql" ]; then
  # fallback when running locally from repo root
  BASE="db/migrations"
fi

case "$CMD" in
  up)
    echo "[migrate] applying $BASE/001_init.sql"
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$BASE/001_init.sql"
    if [ -f "$BASE/002_add_mcq.sql" ]; then
      echo "[migrate] applying $BASE/002_add_mcq.sql"
      psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$BASE/002_add_mcq.sql"
    fi
    ;;
  down)
    echo "[migrate] applying $BASE/001_down.sql"
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$BASE/001_down.sql"
    ;;
  *)
    echo "usage: migrate.sh [up|down]" >&2
    exit 2
    ;;
esac

echo "[migrate] done"
