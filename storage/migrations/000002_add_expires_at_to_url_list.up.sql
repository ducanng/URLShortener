ALTER TABLE url_list
    ADD COLUMN expires_at TIMESTAMPTZ NULL;

CREATE INDEX idx_url_list_expires_at
    ON url_list (expires_at)
    WHERE expires_at IS NOT NULL;
