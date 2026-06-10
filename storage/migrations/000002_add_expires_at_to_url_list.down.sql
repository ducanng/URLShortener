DROP INDEX IF EXISTS idx_url_list_expires_at;

ALTER TABLE url_list
    DROP COLUMN IF EXISTS expires_at;
