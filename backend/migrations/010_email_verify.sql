-- Подтверждение почты кодом при регистрации: аккаунт создаётся только после ввода кода из письма.
CREATE TABLE pending_registrations (
    email         VARCHAR(254) PRIMARY KEY,
    login         VARCHAR(64)  NOT NULL,
    password_hash TEXT         NOT NULL,
    code_hash     VARCHAR(64)  NOT NULL, -- HMAC-SHA256 кода, сам код не хранится
    attempts      INT          NOT NULL DEFAULT 0,
    resends       INT          NOT NULL DEFAULT 0,
    expires_at    TIMESTAMPTZ  NOT NULL,
    sent_at       TIMESTAMPTZ  NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- когда почта подтверждена кодом (NULL — не подтверждалась: создана администратором или до этой версии)
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;
