-- 002_field_guide.sql
--
-- Review field-guide metadata. Nullable columns let existing databases migrate in place; the
-- seed loader fills the catalogue rows and checkpoint helper text on the next boot.

ALTER TABLE metric_defs ADD COLUMN slug TEXT;
ALTER TABLE metric_defs ADD COLUMN definition TEXT;
ALTER TABLE metric_defs ADD COLUMN how_to_measure TEXT;

CREATE UNIQUE INDEX idx_metric_defs_slug ON metric_defs (slug) WHERE slug IS NOT NULL;

ALTER TABLE checkpoints ADD COLUMN helpers TEXT;
