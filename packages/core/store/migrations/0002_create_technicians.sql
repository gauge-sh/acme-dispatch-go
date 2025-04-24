CREATE TABLE technicians (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    phone TEXT,
    skill_tags TEXT NOT NULL DEFAULT '[]',
    hourly_cost_cents INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
