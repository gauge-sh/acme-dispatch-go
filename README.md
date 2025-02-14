# Acme Dispatch

Field-service operations for a small shop: schedule jobs, assign technicians,
track parts, and record what to bill. Internal tool — users are created by an
admin, there is no public signup.

## Quickstart

```
make seed
make web
```

Then sign in at http://localhost:4000 with `admin@acme-dispatch.example` /
`dispatch-admin`. `make api` starts the JSON API on port 4400.

## Layout

| Path | What lives there |
|---|---|
| `packages/core` | The domain: status machine, scheduling, stock, pricing. Plus the SQLite store, migrations, and seeds. No HTTP. |
| `apps/api` | JSON REST API under `/api/v1`, bearer-token auth. |
| `apps/web` | Server-rendered UI: dashboard, jobs, scheduling, parts, invoicing, user admin. |

`go.work` ties the three modules together; run `go` commands from the repo
root or use the Makefile.

## Development

```
make test    # all modules
make lint    # gofmt + go vet
```

See `docs/runbook.md` for the longer version, `docs/architecture.md` for why
the boundaries sit where they do, and `docs/domain.md` for the behavioral
contract the code implements.
