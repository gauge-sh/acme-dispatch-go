CREATE TABLE invoice_records (
    id INTEGER PRIMARY KEY,
    job_id INTEGER NOT NULL UNIQUE REFERENCES jobs(id),
    labor_cents INTEGER NOT NULL,
    parts_cents INTEGER NOT NULL,
    total_cents INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE invoice_line_items (
    id INTEGER PRIMARY KEY,
    invoice_record_id INTEGER NOT NULL REFERENCES invoice_records(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    kind TEXT NOT NULL,
    description TEXT NOT NULL,
    quantity REAL NOT NULL,
    unit_cents INTEGER NOT NULL,
    amount_cents INTEGER NOT NULL,
    weekend INTEGER NOT NULL DEFAULT 0,
    UNIQUE (invoice_record_id, position)
);
