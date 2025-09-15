# Pricing

How an invoice record is priced when a completed job is invoiced. The
authoritative fixture is `packages/core/testdata/pricing-cases.json`; the
conformance test in `packages/core` drives every case in it.

All money is integer cents. Rounding is always **half away from zero to a
whole cent**, applied after each step below — never banker's rounding, and no
fractional cents are carried between steps.

## Labor

Each appointment on the job becomes one labor line:

    hours × technician hourly cost × 1.6

rounded to whole cents. Hours are the exact duration from `scheduled_start`
to `scheduled_end` — a 20-minute visit bills a third of an hour, even though
the line's description reads `0.33h`.

If the appointment **starts on a Saturday or Sunday in UTC**, the
already-rounded line is multiplied by 1.15 and rounded again. The surcharge
is per appointment, not per invoice: a job with one weekday and one Sunday
appointment surcharges only the Sunday line.

Labor lines are ordered by scheduled start, then appointment id.

## Parts

Each job part becomes one part line: `quantity × unit_price_cents`, using the
price captured when the part was recorded on the job — a later price change
never restates an invoice. Whole cents times a whole quantity, so nothing to
round. Part lines follow the labor lines, in job-part id order.

## Total

    total = labor + parts

If the job's priority is `urgent`, the rounded total is multiplied by 1.10
and rounded once more. This is the last step and applies only to the total;
line amounts stay as billed. `low` and `normal` add nothing.

## Worked example (fixture case 4)

1.5h at 8000¢/h: 1.5 × 8000 × 1.6 = 19200. Three parts at 1299¢: 3897.
Subtotal 23097; urgent: 23097 × 1.1 = 25406.7 → **25407**.

## Where it happens

`core.PriceInvoice` computes everything from plain values. The store's
`CreateInvoiceRecord` loads the job's appointments and parts, prices them,
writes the record and its line items, and moves the job to `invoiced`, all in
one transaction. Invoice records are immutable after that.
