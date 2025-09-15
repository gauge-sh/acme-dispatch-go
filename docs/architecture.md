# Architecture

Three modules in one `go.work` workspace. The boundary rule is simple: rules
live in `packages/core`, HTTP lives in the apps, and nothing above core knows
SQL exists.

## packages/core

Two layers in one module.

The root package (`core`) is the domain: entities, enums with the exact wire
values, the job status machine, the half-open appointment overlap rule, stock
arithmetic, field validation, and the pricing engine. It imports nothing but
the standard library — no HTTP, no SQL — and it is where the rules worth
unit-testing exhaustively live. Money is int64 cents end to end; every
fractional factor (hours, the 1.6 markup, the 15% and 10% surcharges) is
applied as an exact integer ratio with a single half-away-from-zero division
helper, so no float ever touches an amount.

`core/store` is the persistence layer: sqlx over file-backed SQLite, embedded
migrations, and the fixed seed dataset. Store methods enforce the rules that
need to see the database — uniqueness, the technician conflict query,
stock-and-job-part transactionality, invoice creation — by calling into the
domain package for every decision it can make on plain values. Both apps
share it, which is why it lives here rather than in either app.

The split within one module (instead of a fourth module) keeps `go.work`
honest: the domain package still cannot import the store, because that would
be an import cycle the compiler rejects.

## apps/api

The JSON API from `docs/domain.md`'s companion contract: bearer-token auth,
CRUD for every entity, transition and invoicing endpoints, pagination with a
capped `per_page`, and one error shape. Handlers translate HTTP to store
calls and domain errors back to `{"error": {"code", "message"}}` bodies; they
hold no rules of their own. Integration tests run the whole handler stack
against a temp-file SQLite database — no network, no mocks.

## apps/web

The server-rendered UI: `net/http` mux, `html/template` pages embedded in the
binary, gorilla/sessions for login. Forms post back, errors re-render the
form with a field-level message, successes redirect with a flash. The web
layer reuses the same store, so anything the API refuses the UI refuses for
free.

## Choices worth a sentence

- **SQLite via modernc.org/sqlite**: pure Go, so `go test` needs no C
  toolchain and the build is the same on every machine.
- **One pooled connection** (`SetMaxOpenConns(1)`): SQLite allows one writer;
  at this tool's scale, queueing on one connection beats handling
  `SQLITE_BUSY` everywhere.
- **Timestamps as RFC3339 UTC text**: fixed-width strings compare correctly
  in SQL, which is what the overlap query relies on.
- **Migrations as embedded .sql files**: applied in filename order into a
  `schema_migrations` table at startup; no external tool to install.
