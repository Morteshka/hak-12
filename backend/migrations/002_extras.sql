-- Дополнительные функции: email/Telegram у пользователей, чек к оплате, бюджеты,
-- настройки интеграций (webhook, Discord), доставки уведомлений по каналам, расширенный журнал.
-- Только ALTER/CREATE: существующие данные сохраняются.

ALTER TABLE users ADD COLUMN email VARCHAR(254);
ALTER TABLE users ADD COLUMN email_notifications BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN telegram_id BIGINT;
CREATE UNIQUE INDEX uq_users_email ON users (lower(email)) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX uq_users_telegram ON users (telegram_id) WHERE telegram_id IS NOT NULL;

-- Ссылка на чек / квитанцию к конкретному периоду
ALTER TABLE payment_occurrences ADD COLUMN receipt_url VARCHAR(1000);

-- Бюджет: плановая сумма на месяц или год по каждой валюте.
-- user_id NULL — общий бюджет команды (платежи в системе общие).
CREATE TABLE budgets (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT        REFERENCES users(id) ON DELETE CASCADE,
    currency   CHAR(3)       NOT NULL CHECK (currency IN ('RUB', 'USD', 'EUR')),
    period     VARCHAR(8)    NOT NULL CHECK (period IN ('month', 'year')),
    amount     NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_budgets ON budgets ((COALESCE(user_id, 0)), currency, period);

-- Глобальные настройки интеграций (webhook_url, discord_webhook_url)
CREATE TABLE app_settings (
    key        VARCHAR(64) PRIMARY KEY,
    value      TEXT        NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Какие напоминания уже отправлены по каким каналам (email:<user_id>, webhook, discord).
-- Первичный ключ = защита от дублей. Telegram по-прежнему использует reminder_sent_at.
CREATE TABLE notification_deliveries (
    occurrence_id BIGINT      NOT NULL REFERENCES payment_occurrences(id) ON DELETE CASCADE,
    channel       VARCHAR(64) NOT NULL,
    sent_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (occurrence_id, channel)
);

-- Журнал: user_agent, детали в JSONB, названия действий как в спецификации
ALTER TABLE audit_logs ADD COLUMN user_agent VARCHAR(512) NOT NULL DEFAULT '';
ALTER TABLE audit_logs ALTER COLUMN details DROP DEFAULT;
ALTER TABLE audit_logs ALTER COLUMN details TYPE JSONB
    USING CASE WHEN details = '' THEN '{}'::jsonb ELSE jsonb_build_object('text', details) END;
ALTER TABLE audit_logs ALTER COLUMN details SET DEFAULT '{}'::jsonb;
ALTER TABLE audit_logs RENAME COLUMN entity TO entity_type;
UPDATE audit_logs SET action = CASE action
    WHEN 'payment_create'   THEN 'create_payment'
    WHEN 'payment_delete'   THEN 'delete_payment'
    WHEN 'payment_pay'      THEN 'mark_paid'
    WHEN 'payment_unpay'    THEN 'unmark_paid'
    WHEN 'payment_fix_date' THEN 'edit_date'
    WHEN 'user_create'      THEN 'create_user'
    WHEN 'user_update'      THEN 'update_user'
    WHEN 'user_delete'      THEN 'delete_user'
    ELSE action END;
CREATE INDEX idx_audit_action ON audit_logs (action);
CREATE INDEX idx_audit_user ON audit_logs (user_id)
