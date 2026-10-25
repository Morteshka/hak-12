-- Каталог услуг и личные подписки.
-- Администратор публикует услуги (цена, валюта, периодичность, пробный период).
-- Пользователь сам подключает нужные: подписка — это обычный регулярный платёж с владельцем.
-- Платежи без владельца (user_id IS NULL) — общие платежи команды, как и раньше.

CREATE TABLE services (
    id          BIGSERIAL PRIMARY KEY,
    title       VARCHAR(200)  NOT NULL,
    description TEXT          NOT NULL DEFAULT '',
    amount      NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    currency    CHAR(3)       NOT NULL CHECK (currency IN ('RUB', 'USD', 'EUR')),
    period      VARCHAR(16)   NOT NULL DEFAULT 'monthly' CHECK (period IN ('monthly', 'quarterly', 'yearly')),
    -- 0 — первый платёж сразу при подключении, N — пробный период N дней, первый платёж после него
    trial_days  SMALLINT      NOT NULL DEFAULT 0 CHECK (trial_days BETWEEN 0 AND 365),
    active      BOOLEAN       NOT NULL DEFAULT TRUE, -- скрытая услуга не видна в каталоге, подписки продолжают работать
    created_by  BIGINT        REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

ALTER TABLE recurring_payments ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE recurring_payments ADD COLUMN service_id BIGINT REFERENCES services(id) ON DELETE SET NULL;
ALTER TABLE recurring_payments ADD COLUMN trial_until DATE;
ALTER TABLE recurring_payments ADD CONSTRAINT recurring_period_check CHECK (period IN ('monthly', 'quarterly', 'yearly'));

-- одна активная подписка пользователя на одну услугу
CREATE UNIQUE INDEX uq_active_subscription ON recurring_payments (user_id, service_id)
    WHERE deleted_at IS NULL AND service_id IS NOT NULL;
CREATE INDEX idx_recurring_user ON recurring_payments (user_id);

-- удаление пользователя убирает его личные подписки вместе с их периодами
ALTER TABLE payment_occurrences DROP CONSTRAINT IF EXISTS payment_occurrences_recurring_payment_id_fkey;
ALTER TABLE payment_occurrences ADD CONSTRAINT payment_occurrences_recurring_payment_id_fkey
    FOREIGN KEY (recurring_payment_id) REFERENCES recurring_payments(id) ON DELETE CASCADE;

-- бюджеты теперь личные: у каждого свой план, прежний общий бюджет достаётся администратору из .env
UPDATE budgets SET user_id = (SELECT id FROM users WHERE is_system ORDER BY id LIMIT 1) WHERE user_id IS NULL;
