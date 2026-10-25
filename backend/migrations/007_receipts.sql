-- Проверка оплат по чекам.
-- Пользователь прикладывает чек (фото, скриншот или PDF) на сайте или в Telegram-боте.
-- Система распознаёт чек (QR кассового чека, текст PDF, OCR, при наличии ключа — ИИ-модель),
-- сверяет сумму, валюту, дату и уникальность и либо подтверждает оплату сама, либо отдаёт администратору.
-- Чеки и решения хранятся всегда: при удалении платежа или пользователя запись остаётся (снимок данных платежа).

CREATE TABLE payment_receipts (
    id                BIGSERIAL PRIMARY KEY,
    occurrence_id     BIGINT        REFERENCES payment_occurrences(id) ON DELETE SET NULL,
    user_id           BIGINT        REFERENCES users(id) ON DELETE SET NULL,
    submitted_by      VARCHAR(100)  NOT NULL DEFAULT '',
    source            VARCHAR(16)   NOT NULL DEFAULT 'web' CHECK (source IN ('web', 'telegram')),

    -- снимок платежа на момент загрузки (история не теряется при удалении)
    payment_title     VARCHAR(200)  NOT NULL DEFAULT '',
    payment_amount    NUMERIC(14,2) NOT NULL DEFAULT 0,
    payment_currency  CHAR(3)       NOT NULL DEFAULT 'RUB',
    payment_due_date  DATE,
    owner_login       VARCHAR(100),

    -- файл
    file_name         VARCHAR(255)  NOT NULL DEFAULT '',
    mime              VARCHAR(100)  NOT NULL,
    size              INT           NOT NULL,
    sha256            CHAR(64)      NOT NULL,
    content           BYTEA         NOT NULL,
    note              TEXT          NOT NULL DEFAULT '',

    -- что распознано
    methods           VARCHAR(64)   NOT NULL DEFAULT '',
    ext_amount        NUMERIC(14,2),
    ext_currency      CHAR(3),
    ext_date          DATE,
    ext_operation_id  VARCHAR(200),
    ext_payee         VARCHAR(300),
    ext_text          TEXT          NOT NULL DEFAULT '',
    ai_comment        TEXT          NOT NULL DEFAULT '',

    -- результат автопроверки: [{code, status: ok|warn|fail, title, detail}]
    checks            JSONB         NOT NULL DEFAULT '[]'::jsonb,
    score             SMALLINT      NOT NULL DEFAULT 0,

    status            VARCHAR(16)   NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    decided_by        VARCHAR(100),
    decided_auto      BOOLEAN       NOT NULL DEFAULT FALSE,
    decision_comment  TEXT          NOT NULL DEFAULT '',
    decided_at        TIMESTAMPTZ,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_receipts_occurrence ON payment_receipts (occurrence_id, id DESC);
CREATE INDEX idx_receipts_status ON payment_receipts (status, created_at DESC);
CREATE INDEX idx_receipts_user ON payment_receipts (user_id);
CREATE INDEX idx_receipts_sha ON payment_receipts (sha256);
CREATE INDEX idx_receipts_operation ON payment_receipts (ext_operation_id) WHERE ext_operation_id IS NOT NULL;
