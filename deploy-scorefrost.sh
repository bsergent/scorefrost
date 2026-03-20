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

echo "==> Starting database..."
compose up -d db

echo "==> Waiting for database readiness..."
until compose exec -T db pg_isready -U "$PGUSER" -d "$PGDB" >/dev/null 2>&1; do
  sleep 1
done

echo "==> Running migrations..."
compose run --rm --no-deps migrate

echo "==> Rebuilding and restarting API..."
compose up -d --build --remove-orphans api

echo "==> Pruning old images..."
docker image prune -f || true

echo "==> Deployment complete."
