-- +goose Up
CREATE TABLE notifications (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id       UUID        NOT NULL UNIQUE,
    correlation_id VARCHAR(100),
    event_type     VARCHAR(100) NOT NULL,
    member_id      VARCHAR(100),
    book_id        UUID,
    book_title     VARCHAR(255),
    message        TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE processed_events (
    event_id     UUID        PRIMARY KEY,
    event_type   VARCHAR(100) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE processed_events;
DROP TABLE notifications;
