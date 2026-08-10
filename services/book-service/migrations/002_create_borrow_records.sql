-- +goose Up
CREATE TABLE borrow_records (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    book_id     UUID        NOT NULL REFERENCES books(id),
    member_id   VARCHAR(100) NOT NULL,
    member_name VARCHAR(255),
    borrowed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    due_date    TIMESTAMPTZ NOT NULL,
    returned_at TIMESTAMPTZ,
    status      VARCHAR(20) NOT NULL DEFAULT 'borrowed'
);

-- +goose Down
DROP TABLE borrow_records;
