# Acme Dispatch — domain contract

This is the frozen behavioral contract for every Acme Dispatch implementation.
Ship it as `docs/domain.md` in each mature repo, verbatim. Field names below are
the **wire** names (snake_case). Storage column names follow each stack's
convention; the wire format does not.

Money is always an integer count of cents. Timestamps are ISO-8601 in UTC with a
`Z` suffix. Ids are integers assigned by the database.

Every entity carries `id`, `created_at`, and `updated_at`. They are omitted from
the field tables below.

## Enums

| Enum | Values |
|---|---|
| `role` | `admin`, `dispatcher` |
| `priority` | `low`, `normal`, `urgent` |
| `job_status` | `requested`, `scheduled`, `in_progress`, `completed`, `invoiced`, `cancelled` |

Enum values are lower-case snake_case on the wire in every stack, including the
Rails and Go implementations.

## User

| Field | Type | Null? | Rules |
|---|---|---|---|
| `email` | string | no | unique, case-insensitive, must contain `@` |
| `name` | string | no | 1–120 chars after trimming |
| `role` | `role` | no | defaults to `dispatcher` |
| `is_active` | bool | no | defaults to `true`; inactive users cannot log in |

Password is managed by the stack's auth product and is never serialized. The API
never returns a password field or hash.

- Only an `admin` may create, update, or delete users, or issue API tokens.
- A user may not change their own `role`, and may not deactivate themselves.
- Deleting the last active admin is rejected with `validation_failed` /
  `"cannot remove the last active admin"`. Demoting or deactivating the last
  active admin causes the same lockout and is rejected with the same code and
  message.
- Deleting yourself is allowed over the API when you are not the last active
  admin. The web UI refuses self-deletion outright.

## ApiToken

| Field | Type | Null? | Rules |
|---|---|---|---|
| `user_id` | int | no | owner |
| `name` | string | no | 1–80 chars |
| `token_hash` | string | no | never serialized |
| `last_used_at` | timestamp | yes | updated on each authenticated request |

Issued from an admin screen. The plaintext token is shown exactly once, on
creation, as `token` in the creation response only. Store a hash (the stack's
canonical one-way digest), never the plaintext.

## Customer

| Field | Type | Null? | Rules |
|---|---|---|---|
| `name` | string | no | 1–160 chars after trimming |
| `contact_email` | string | yes | must contain `@` when present |
| `phone` | string | yes | 7–32 chars when present |
| `service_address` | string | no | 1–240 chars |
| `notes` | string | yes | up to 2000 chars |

Deleting a customer that has any job is rejected with `validation_failed` /
`"customer has jobs and cannot be deleted"`.

## Technician

| Field | Type | Null? | Rules |
|---|---|---|---|
| `name` | string | no | 1–120 chars |
| `phone` | string | yes | 7–32 chars when present |
| `skill_tags` | string[] | no | may be empty; each tag 1–40 chars, lower-case, deduplicated, stored sorted |
| `hourly_cost_cents` | int | no | > 0 |
| `is_active` | bool | no | defaults to `true` |

- `skill_tags` serializes as a JSON array of strings in every stack.
- An inactive technician cannot be assigned to a new appointment:
  `validation_failed` / `"technician is not active"`.
- Deleting a technician that has any appointment is rejected with
  `validation_failed` / `"technician has appointments and cannot be deleted"`.

## Job

| Field | Type | Null? | Rules |
|---|---|---|---|
| `customer_id` | int | no | must exist |
| `title` | string | no | 1–160 chars |
| `description` | string | yes | up to 4000 chars |
| `priority` | `priority` | no | defaults to `normal` |
| `status` | `job_status` | no | starts at `requested`; only changed through a transition |

### Status machine

```
requested ──▶ scheduled ──▶ in_progress ──▶ completed ──▶ invoiced
     │             │              │
     └─────────────┴──────────────┴──▶ cancelled
```

- Legal transitions and nothing else: `requested→scheduled`,
  `scheduled→in_progress`, `in_progress→completed`, `completed→invoiced`,
  and `→cancelled` from `requested`, `scheduled`, or `in_progress`.
- `cancelled` and `invoiced` are terminal.
- `completed→invoiced` is performed **only** by invoice creation
  (`POST /jobs/{id}/invoice`), never by the transition endpoint. Asking the
  transition endpoint for `invoiced` is `invalid_transition`.
- Any other transition is rejected with `invalid_transition` /
  `"cannot move a job from <from> to <to>"`.
- `requested→scheduled` requires at least one appointment:
  `validation_failed` / `"a job needs an appointment before it can be scheduled"`.
- `status` is never writable through `POST /jobs` or `PATCH /jobs/{id}`; those
  requests ignore the field rather than erroring.

## Appointment

| Field | Type | Null? | Rules |
|---|---|---|---|
| `job_id` | int | no | must exist; job must not be `completed`, `invoiced`, or `cancelled` |
| `technician_id` | int | no | must exist and be active |
| `scheduled_start` | timestamp | no | — |
| `scheduled_end` | timestamp | no | strictly after `scheduled_start` |
| `completion_notes` | string | yes | up to 2000 chars |

- **Conflict rule.** Two appointments for the same technician may not overlap.
  Overlap is half-open: `[start, end)`. Touching endpoints do not overlap
  (`09:00–10:00` and `10:00–11:00` are fine). Violations are
  `schedule_conflict` / `"technician is already booked for that time"`, HTTP 409,
  on both create and update.
- `scheduled_end <= scheduled_start` is `validation_failed` /
  `"appointment must end after it starts"`.
- Attaching an appointment to a `completed`, `invoiced`, or `cancelled` job is
  `validation_failed` /
  `"appointments cannot be added to a job that is <status>"`, where `<status>`
  is the wire enum value. "Attaching" means creating an appointment, or
  re-pointing an existing one at another job — editing the times or
  `completion_notes` of an appointment already on a closed job stays allowed,
  because that is how completion notes get written after the work is done.
- `hours` for pricing is `(scheduled_end - scheduled_start)` in hours as an exact
  decimal, not rounded to any granularity.

## Part

| Field | Type | Null? | Rules |
|---|---|---|---|
| `sku` | string | no | unique, 1–40 chars, upper-case alphanumerics and dashes |
| `name` | string | no | 1–160 chars |
| `unit_cost_cents` | int | no | > 0 |
| `unit_price_cents` | int | no | > 0 and `>= unit_cost_cents` |

`unit_price_cents < unit_cost_cents` is `validation_failed` /
`"unit price cannot be below unit cost"`. The rule is enforced on create and on
update.

Deleting a part that has been used on any job is rejected with
`validation_failed` / `"part has been used on jobs and cannot be deleted"`;
billing history must stay intact. An unused part deletes cleanly, taking its
zero stock level with it.

## StockLevel

| Field | Type | Null? | Rules |
|---|---|---|---|
| `part_id` | int | no | unique — exactly one stock level per part |
| `quantity_on_hand` | int | no | `>= 0` |

- A stock level row is created with the part, at `quantity_on_hand = 0`.
- Adjustment (`POST /stock-levels/{part_id}/adjust`, body `{"delta": n}`) with a
  non-zero integer delta. A result below zero is rejected with
  `insufficient_stock` / `"not enough stock on hand"`, HTTP 409, and nothing is
  written. `delta: 0` is `validation_failed` / `"delta must not be zero"`.

## JobPart

| Field | Type | Null? | Rules |
|---|---|---|---|
| `job_id` | int | no | must exist; job must not be `invoiced` or `cancelled` |
| `part_id` | int | no | must exist |
| `quantity` | int | no | > 0 |
| `unit_price_cents` | int | no | copied from the part at the time of use |

- Recording a job part decrements that part's stock by `quantity` in the same
  transaction; insufficient stock rejects the whole request with
  `insufficient_stock` (HTTP 409) and decrements nothing.
- `unit_price_cents` is captured on write so a later price change does not
  restate an existing invoice.
- Recording the same part twice on one job creates two rows; they are not merged.
- Attaching a part to an `invoiced` or `cancelled` job is `validation_failed` /
  `"parts cannot be added to a job that is <status>"`, where `<status>` is the
  wire enum value.
- `quantity <= 0` is `validation_failed` /
  `"quantity must be greater than zero"`.

## InvoiceRecord

| Field | Type | Null? | Rules |
|---|---|---|---|
| `job_id` | int | no | unique — one invoice record per job |
| `labor_cents` | int | no | sum of labor line items |
| `parts_cents` | int | no | sum of part line items |
| `total_cents` | int | no | final total after the urgent multiplier |
| `line_items` | LineItem[] | no | ordered: all labor lines (by `scheduled_start` ascending, then appointment id ascending), then all part lines (by job-part id ascending) |

### LineItem

| Field | Type | Null? | Notes |
|---|---|---|---|
| `kind` | `labor` \| `part` | no | — |
| `description` | string | no | labor: `"<technician name> — <hours>h on <YYYY-MM-DD>"`; part: `"<sku> — <name>"` |
| `quantity` | decimal | no | labor: hours; part: quantity used |
| `unit_cents` | int | no | labor: technician `hourly_cost_cents`; part: captured `unit_price_cents` |
| `amount_cents` | int | no | the rounded line total, weekend surcharge included |
| `weekend` | bool | no | labor lines only; always `false` on part lines |

- Creation is allowed only from `completed`; from any other status it is
  `invalid_transition` / `"only a completed job can be invoiced"`.
- Creation moves the job to `invoiced` in the same transaction.
- A second invoice for the same job is `validation_failed` /
  `"job is already invoiced"`.
- **Precedence.** An already-invoiced job satisfies both rules above, and they
  return different HTTP statuses, so the order is observable: the duplicate
  check runs **first**. Invoicing a job that already has a record is
  `validation_failed` (400). `invalid_transition` (409) is reserved for a job
  with no record whose status is not `completed`.
- Invoice records are immutable: no update or delete endpoint exists.

`<hours>` in a labor description is the duration quantized to 2 decimal places
(half away from zero), with trailing zeros and any trailing decimal point
stripped, never in exponent notation: `2h`, `1.5h`, `1h`, `0.25h`, `0.33h`,
`0.3h`. That label is display only — the amount is always computed from the
exact duration, so a 20-minute visit bills one third of an hour, not 0.33.

`quantity` serializes as a bare JSON number, not a string: `1.5` for a
fractional labor line, `2` for a whole-hour one, `3` for a part line. Never
`3.0`.

## Pricing

Frozen, and driven by `packages/core/testdata/pricing-cases.json` (copy that file
verbatim; drive one parameterized test from it).

1. **Labor line**, per appointment: `hours × technician.hourly_cost_cents × 1.6`,
   rounded to whole cents.
2. **Weekend surcharge**: if the appointment's `scheduled_start` falls on a
   Saturday or Sunday **in UTC**, multiply the already-rounded labor line by
   `1.15` and round again. This is **per appointment**, not per invoice — a job
   with one weekday and one Sunday appointment surcharges only the Sunday line.
   `specs/acme-dispatch.md` describes it loosely as a flat percentage "when any
   appointment falls on Sat/Sun"; that prose is wrong and this rule wins.
   Fixture case 5 is the discriminator: 16000 + 9200 = **25200**, not
   24000 × 1.15 = 27600.
3. **Part line**, per job part: `quantity × unit_price_cents`. Exact; no
   rounding needed.
4. **Total**: `labor_cents + parts_cents`.
5. **Urgent**: if the job's priority is `urgent`, multiply the rounded total by
   `1.10` and round. Applied last, to the total only — never to individual lines.
6. `low` and `normal` priority apply no multiplier.

**Rounding is round-half-away-from-zero to whole cents, applied after each line
computation and after the urgent multiplier.** Do not use banker's rounding, and
do not carry fractional cents between steps. Implement it explicitly — most
stacks' default `round` is either banker's rounding or float-fragile.

Only appointments and job parts attached to the job at invoice time are billed.
Appointment `completion_notes` do not affect pricing.

## Seed data

Deterministic, from a fixed seed, in every repo: 2 users (one `admin`, one
`dispatcher`), ~8 technicians, ~20 customers, ~40 jobs spread across all six
statuses, ~25 parts with stock levels, job parts on the worked jobs, and invoice
records for the `invoiced` jobs.

Determinism is per repo: re-running the seed must reproduce the same rows and
the same invoice totals every time. The dataset itself is each repo's own — the
five are not required to seed identical customers or identical totals. Pricing
agreement across implementations is proven by `pricing-cases.json`, which is
byte-identical everywhere, not by the seeds.

Seed durations should be whole or terminating fractions of an hour, so a labor
line's `quantity` never depends on floating-point formatting.
