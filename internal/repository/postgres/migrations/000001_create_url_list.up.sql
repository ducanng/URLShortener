CREATE TABLE IF NOT EXISTS url_list
(
    id           BIGINT       NOT NULL
        CONSTRAINT shorten_id PRIMARY KEY,
    original_url TEXT         NOT NULL,
    shorted_url  VARCHAR(16)  NOT NULL,
    clicks       INT          NOT NULL
);
