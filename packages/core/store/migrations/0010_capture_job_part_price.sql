-- Capture the sale price at the moment a part is used, so a later price
-- change never restates an existing job or invoice. Existing rows backfill
-- from the part's current price, the best information available.
ALTER TABLE job_parts ADD COLUMN unit_price_cents INTEGER;

UPDATE job_parts
SET unit_price_cents = (SELECT unit_price_cents FROM parts WHERE parts.id = job_parts.part_id);
