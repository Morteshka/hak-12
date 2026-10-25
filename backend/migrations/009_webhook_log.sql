-- Журнал отправок webhook: что, куда и с каким результатом ушло (последние 200 записей).
-- Видно администратору в «Настройки → Webhook», чтобы интеграцию можно было проверить без логов сервера.
CREATE TABLE webhook_deliveries (
    id          BIGSERIAL PRIMARY KEY,
    event       VARCHAR(48)  NOT NULL,
    payment_id  BIGINT,
    name        VARCHAR(200) NOT NULL DEFAULT '',
    url         VARCHAR(500) NOT NULL,
    ok          BOOLEAN      NOT NULL,
    status_code INT          NOT NULL DEFAULT 0,
    duration_ms INT          NOT NULL DEFAULT 0,
    error       VARCHAR(500) NOT NULL DEFAULT '',
    payload     TEXT         NOT NULL,
    response    VARCHAR(500) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
