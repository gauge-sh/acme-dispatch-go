# Runbook

## Prerequisites

Go 1.27 or newer. Nothing else — the SQLite driver is pure Go and the
database is a file.

## First run

```
make seed    # creates dispatch.db, runs migrations, loads the fixed dataset
make web     # http://localhost:4000
```

Seeded logins:

| Email | Password | Role |
|---|---|---|
| `admin@acme-dispatch.example` | `dispatch-admin` | admin |
| `office@acme-dispatch.example` | `dispatch-office` | dispatcher |

`make api` starts the JSON API on port 4400 against the same database. API
requests need a bearer token: sign into the web app as the admin, open
Users → pick a user → issue a token, and copy it when it is shown (that is
the only time it appears).

```
curl -H "Authorization: Bearer <token>" localhost:4400/api/v1/jobs
curl -H "Authorization: Bearer <token>" "localhost:4400/api/v1/jobs?status=completed&per_page=50"
curl -H "Authorization: Bearer <token>" -X POST localhost:4400/api/v1/jobs/12/invoice
```

Lists take `?page` (1-based) and `?per_page` (default 20, max 100) and reply
with `{"items", "page", "per_page", "total"}`. Errors always look like
`{"error": {"code": "schedule_conflict", "message": "..."}}`.

## Day to day

```
make test    # full suite, all three modules, no network needed
make lint    # gofmt + go vet
make fmt     # rewrite formatting in place
make clean   # delete the local database
```

Both binaries take flags directly if the Makefile defaults don't fit:

```
go run ./apps/web -addr :8080 -db /tmp/scratch.db
go run ./apps/api -db /tmp/scratch.db
```

`DISPATCH_DB` sets the database path when a flag is awkward. `SESSION_SECRET`
must be set to a real value anywhere other people can reach the web app;
without it a fixed development key is used.

## Database

Migrations are embedded in the binaries and apply automatically on startup,
tracked in the `schema_migrations` table. To reset everything:

```
make clean && make seed
```

Re-seeding wipes all rows and reloads the same fixed dataset, so invoice
totals and ids come out identical every time.

## Tests

Each module carries its own tests: domain rules and pricing conformance in
`packages/core`, HTTP integration in `apps/api`, browser-level smoke tests in
`apps/web`. Every test opens its own temp-file database, so they run in
parallel and leave nothing behind.
