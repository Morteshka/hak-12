-- Автопродление, разовые платежи и уведомления внутри веб-панели (колокольчик)

-- true: создаются следующие периоды, false: подписка не продлевается
ALTER TABLE recurring_payments ADD COLUMN auto_renew BOOLEAN NOT NULL DEFAULT TRUE;

-- разовый платёж (once): один период без продления
ALTER TABLE recurring_payments DROP CONSTRAINT recurring_period_check;
ALTER TABLE recurring_payments ADD CONSTRAINT recurring_period_check CHECK (period IN ('monthly', 'quarterly', 'yearly', 'once'));

-- Уведомления для колокольчика: у каждого пользователя свои, прочитанные помечаются
CREATE TABLE notifications (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        VARCHAR(32)  NOT NULL,
    level       VARCHAR(16)  NOT NULL DEFAULT 'info',
    title       VARCHAR(200) NOT NULL,
    body        VARCHAR(1000) NOT NULL DEFAULT '',
    occurrence_id BIGINT     REFERENCES payment_occurrences(id) ON DELETE SET NULL,
    receipt_id  BIGINT,
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_user ON notifications (user_id, id DESC);
CREATE INDEX idx_notifications_unread ON notifications (user_id) WHERE read_at IS NULL;
