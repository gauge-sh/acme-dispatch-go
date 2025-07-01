-- Technicians with appointment history cannot be deleted; deactivation is
-- how someone leaves the roster.
ALTER TABLE technicians ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1;
