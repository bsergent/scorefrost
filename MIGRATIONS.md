# Database Migrations

This project now includes versioned SQL migrations in `sql/migrations`.

## Tooling choice

Use [golang-migrate](https://github.com/golang-migrate/migrate) for applying migrations.

A migration version table (`schema_migrations`) is created automatically by the migration tool.

## Current layout

- `sql/migrations/000001_initial_schema.up.sql`
- `sql/migrations/000001_initial_schema.down.sql`
- `sql/migrations/000002_add_game_version_to_user_creation.up.sql`
- `sql/migrations/000002_add_game_version_to_user_creation.down.sql`
- `sql/migrations/000003_migrate_solution_id_to_uuid_v7.up.sql`
- `sql/migrations/000003_migrate_solution_id_to_uuid_v7.down.sql`
- `sql/migrations/000004_get_best_scores_optional_levels.up.sql`
- `sql/migrations/000004_get_best_scores_optional_levels.down.sql`

## Runtime behavior

The API no longer applies schema files at startup.

Schema changes must be applied via migration files before starting/upgrading the API.

## Running migrations (Dockerized CLI)

Use a one-off container so you do not need to install the binary globally.

### Linux/macOS shell

```bash
export DATABASE_URL="postgres://$PGUSER:$PGPASSWORD@$PGHOST:$PGPORT/$PGDB?sslmode=disable"

docker run --rm \
  -v "$PWD/sql/migrations:/migrations" \
  --network host \
  migrate/migrate \
  -path=/migrations \
  -database "$DATABASE_URL" \
  up
```

### Windows PowerShell

```powershell
$env:DATABASE_URL = "postgres://$env:PGUSER:$env:PGPASSWORD@$env:PGHOST`:$env:PGPORT/$env:PGDB?sslmode=disable"

docker run --rm `
  -v "${PWD}/sql/migrations:/migrations" `
  --network host `
  migrate/migrate `
  -path=/migrations `
  -database "$env:DATABASE_URL" `
  up
```

## Common commands

### Check current version

```bash
migrate -path ./sql/migrations -database "$DATABASE_URL" version
```

### Roll back one migration

```bash
migrate -path ./sql/migrations -database "$DATABASE_URL" down 1
```

### Create a new migration

```bash
migrate create -ext sql -dir sql/migrations -seq add_example_column
```

## First-time adoption notes

If a database was already created by the legacy bootstrap, do one of the following:

1. **Fresh DB (recommended for first production deploy)**
   - Start with an empty database.
   - Run `up` from migration `000001`.

2. **Existing DB already initialized**
   - Keep legacy schema as-is.
   - Mark baseline as applied:
     - `migrate ... force 1`
   - Add future schema changes as `000002+` migrations.

Do not run `000001` against a populated DB unless you understand the effects and verify idempotency.
