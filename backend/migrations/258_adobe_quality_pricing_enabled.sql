-- Adobe groups without a saved mode start in
-- quality billing. An existing mode column (including explicit false) is kept.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema() AND table_name = 'groups'
          AND column_name = 'image_quality_pricing_enabled'
    ) THEN
        ALTER TABLE groups ADD COLUMN image_quality_pricing_enabled BOOLEAN NOT NULL DEFAULT FALSE;
        UPDATE groups SET image_quality_pricing_enabled = TRUE WHERE platform = 'adobe';
    END IF;
END $$;
-- Creation defaults are platform-aware in the service: Adobe true, others false.
COMMENT ON COLUMN groups.image_quality_pricing_enabled IS 'Enable Adobe quality pricing; false retains prices and uses legacy billing. Adobe creation defaults to true in the application';
