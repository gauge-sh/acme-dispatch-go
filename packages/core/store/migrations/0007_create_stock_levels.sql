CREATE TABLE stock_levels (
    id INTEGER PRIMARY KEY,
    part_id INTEGER NOT NULL UNIQUE REFERENCES parts(id) ON DELETE CASCADE,
    quantity_on_hand INTEGER NOT NULL DEFAULT 0 CHECK (quantity_on_hand >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
