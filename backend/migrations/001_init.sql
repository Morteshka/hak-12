-- Начальная схема Invoice Keeper

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    login         VARCHAR(64)  NOT NULL UNIQUE,
    password_hash TEXT         NOT NULL,
    role          VARCHAR(16)  NOT NULL CHECK (role IN ('admin', 'user')),
    is_system     BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id         VARCHAR(64) PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE recurring_payments (
    id             BIGSERIAL PRIMARY KEY,
    title          VARCHAR(200)  NOT NULL,
    amount         NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    currency       CHAR(3)       NOT NULL CHECK (currency IN ('RUB', 'USD', 'EUR')),
    period         VARCHAR(16)   NOT NULL DEFAULT 'monthly',
    day_of_month   SMALLINT      NOT NULL CHECK (day_of_month BETWEEN 1 AND 31),
    first_due_date DATE          NOT NULL,
    created_by     BIGINT        REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);

-- Каждый период платежа — отдельная запись со своей отметкой оплаты.
CREATE TABLE payment_occurrences (
    id                   BIGSERIAL PRIMARY KEY,
    recurring_payment_id BIGINT        NOT NULL REFERENCES recurring_payments(id),
    title                VARCHAR(200)  NOT NULL,
    amount               NUMERIC(14,2) NOT NULL,
    currency             CHAR(3)       NOT NULL,
    due_date             DATE          NOT NULL,
    paid_at              DATE,
    paid_by              VARCHAR(100),
    reminder_sent_at     TIMESTAMPTZ,
    created_at           TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT uq_occurrence_period UNIQUE (recurring_payment_id, due_date)
);

CREATE INDEX idx_occurrences_due ON payment_occurrences (due_date);
CREATE INDEX idx_occurrences_reminder ON payment_occurrences (due_date)
    WHERE paid_at IS NULL AND reminder_sent_at IS NULL;

CREATE TABLE audit_logs (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT,
    actor      VARCHAR(100) NOT NULL DEFAULT '',
    action     VARCHAR(64)  NOT NULL,
    entity     VARCHAR(64)  NOT NULL DEFAULT '',
    entity_id  VARCHAR(64)  NOT NULL DEFAULT '',
    details    TEXT         NOT NULL DEFAULT '',
    ip         VARCHAR(64)  NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_created ON audit_logs (created_at DESC);
