CREATE TABLE appointments (
    id INTEGER PRIMARY KEY,
    job_id INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    technician_id INTEGER NOT NULL REFERENCES technicians(id),
    scheduled_start TEXT NOT NULL,
    scheduled_end TEXT NOT NULL,
    completion_notes TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_appointments_job ON appointments(job_id);
CREATE INDEX idx_appointments_technician ON appointments(technician_id);
