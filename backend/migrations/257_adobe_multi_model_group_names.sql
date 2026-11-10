-- Preserve full model names when several Adobe models are selected.
-- Widening only; existing names and group data are unchanged.
ALTER TABLE groups ALTER COLUMN name TYPE VARCHAR(512);
