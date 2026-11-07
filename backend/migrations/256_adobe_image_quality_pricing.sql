-- Optional prices: canonical Image 2.6 model -> 1K/2K/4K -> effective quality -> USD/image.
-- NULL / missing cells preserve all existing price sources and multipliers.
ALTER TABLE groups ADD COLUMN IF NOT EXISTS image_quality_prices JSONB;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS image_quality VARCHAR(16);
COMMENT ON COLUMN groups.image_quality_prices IS 'Optional Adobe Image 2.6 model/resolution/quality prices in USD per image; missing cells inherit existing billing';
COMMENT ON COLUMN usage_logs.image_quality IS 'Effective native Adobe Image 2.6 quality; historical rows remain NULL';
