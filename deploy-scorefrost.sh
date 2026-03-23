#!/usr/bin/env bash
set -euo pipefail

APP_DIR="/home/scorefrost/scorefrost-server"
BRANCH="production"

compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose "$@"
  else
    echo "ERROR: docker compose / docker-compose not found."
    exit 1
  fi
}

read_env_var() {
  local key="$1"
  local value

  value="$(grep -E "^${key}=" .env | tail -n 1 | cut -d '=' -f 2- || true)"
  value="${value%$'\r'}"

  # Strip optional surrounding single or double quotes.
  value="${value#\"}"
  value="${value%\"}"
  value="${value#\'}"
  value="${value%\'}"

  if [[ -z "$value" ]]; then
    echo "ERROR: Missing required $key in .env"
    exit 1
  fi

  printf '%s' "$value"
}

echo "==> Ensuring repo exists..."
if [[ ! -d "$APP_DIR/.git" ]]; then
  echo "ERROR: $APP_DIR is not a git repository yet. Clone it first."
  exit 1
fi

cd "$APP_DIR"

echo "==> Updating code..."
git fetch origin
git checkout "$BRANCH"
git reset --hard "origin/$BRANCH"

if [[ ! -f ".env" ]]; then
  echo "ERROR: .env not found in $APP_DIR"
  exit 1
fi

PGUSER_VALUE="$(read_env_var PGUSER)"
PGDB_VALUE="$(read_env_var PGDB)"

EXPECTED_MIGRATION_RAW="$({ ls -1 sql/migrations/*.up.sql 2>/dev/null || true; } | xargs -r -n1 basename | sed -E 's/^([0-9]+)_.*$/\1/' | sort -n | tail -1)"
if [[ -z "$EXPECTED_MIGRATION_RAW" ]]; then
  echo "ERROR: Could not determine expected migration version from sql/migrations/*.up.sql"
  exit 1
fi
EXPECTED_MIGRATION_NUM=$((10#$EXPECTED_MIGRATION_RAW))
echo "==> Expected schema migration version: $EXPECTED_MIGRATION_NUM"

echo "==> Starting database..."
compose up -d db

echo "==> Waiting for database readiness..."
until compose exec -T db pg_isready -U "$PGUSER_VALUE" -d "$PGDB_VALUE" >/dev/null 2>&1; do
  sleep 1
done

echo "==> Running migrations..."
compose run --rm --no-deps migrate

echo "==> Verifying migration state..."
CURRENT_MIGRATION="$(compose exec -T db psql -U "$PGUSER_VALUE" -d "$PGDB_VALUE" -Atc "SELECT version::text FROM schema_migrations LIMIT 1;" 2>/dev/null || true)"
CURRENT_DIRTY="$(compose exec -T db psql -U "$PGUSER_VALUE" -d "$PGDB_VALUE" -Atc "SELECT dirty::text FROM schema_migrations LIMIT 1;" 2>/dev/null || true)"

if [[ -z "$CURRENT_MIGRATION" ]]; then
  echo "ERROR: schema_migrations is missing or empty after migration run."
  exit 1
fi

if [[ "$CURRENT_DIRTY" != "f" ]]; then
  echo "ERROR: schema_migrations is dirty (dirty=$CURRENT_DIRTY, version=$CURRENT_MIGRATION)."
  exit 1
fi

CURRENT_MIGRATION_NUM=$((10#$CURRENT_MIGRATION))
if [[ "$CURRENT_MIGRATION_NUM" -ne "$EXPECTED_MIGRATION_NUM" ]]; then
  echo "ERROR: schema version mismatch. expected=$EXPECTED_MIGRATION_NUM current=$CURRENT_MIGRATION_NUM"
  exit 1
fi

echo "==> Migration verification passed (version=$CURRENT_MIGRATION_NUM)."

echo "==> Rebuilding and restarting API..."
compose up -d --build --remove-orphans api

echo "==> Pruning old images..."
docker image prune -f || true

echo "==> Deployment complete."
