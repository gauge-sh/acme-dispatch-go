-- The conflict check scans a technician's appointments on every create and
-- reschedule; the dashboard groups jobs by status on every load.
CREATE INDEX idx_appointments_technician_start ON appointments(technician_id, scheduled_start);
CREATE INDEX idx_jobs_status ON jobs(status);
