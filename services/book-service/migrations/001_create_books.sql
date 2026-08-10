-- +goose Up
CREATE TABLE books (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    title            VARCHAR(255) NOT NULL,
    author           VARCHAR(255) NOT NULL,
    isbn             VARCHAR(20)  NOT NULL UNIQUE,
    total_quantity   INT         NOT NULL CHECK (total_quantity > 0),
    available_count  INT         NOT NULL CHECK (available_count >= 0),
    status           VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE books;
