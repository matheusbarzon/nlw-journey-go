-- Write your migrate up statements here

CREATE TABLE IF NOT EXISTS trips (
    "id"            uuid            PRIMARY KEY NOT NULL    DEFAULT gen_random_uuid(),
    "destination"   VARCHAR(255)                NOT NULL,
    "owner_email"   VARCHAR(255)                NOT NULL,
    "owner_name"    VARCHAR(255)                NOT NULL,
    "is_confirmed"  BOOLEAN                     NOT NULL    DEFAULT FALSE,
    "starts_at"     TIMESTAMP                   NOT NULL,
    "ends_at"       TIMESTAMP                   NOT NULL
);


---- create above / drop below ----

-- Write your migrate down statements here. If this migration is irreversible
-- Then delete the separator line above.

DROP TABLE IF EXISTS trips;