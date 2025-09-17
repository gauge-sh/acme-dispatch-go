# Acme Dispatch

Field-service operations for a small shop: schedule jobs, assign technicians,
track parts and stock, and record what to bill. It is an internal tool —
users are created by an admin, there is no public signup, and an invoice
record is a record, not a payment.

## Quickstart

```
git clone <this repo> && cd acme-dispatch-go
make seed
make web
```

Sign in at http://localhost:4000 as `admin@acme-dispatch.example` /
`dispatch-admin` (or `office@acme-dispatch.example` / `dispatch-office` for
the dispatcher view). `make api` serves the JSON API on port 4400; issue a
bearer token from the Users screen first.

## Layout

| Path | What lives there |
|---|---|
| `packages/core` | The domain: status machine, scheduling conflict rule, stock rules, pricing — plus the SQLite store, migrations, and seeds. No HTTP. |
| `apps/api` | JSON REST API under `/api/v1`: bearer-token auth, CRUD for every entity, transitions, invoicing, pagination. |
| `apps/web` | Server-rendered UI: dashboard, jobs and scheduling, customers, technicians, parts and stock, invoice records, user admin. |

`go.work` ties the modules together; run everything through the Makefile or
plain `go` commands from the repo root.

## Development

```
make test    # all three modules, offline, file-backed SQLite
make lint    # gofmt + go vet
```

Longer version in `docs/runbook.md`. `docs/architecture.md` explains where
the boundaries are and why; `docs/pricing.md` walks the billing math;
`docs/domain.md` is the behavioral contract the code implements.
