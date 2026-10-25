-- Подтверждение Telegram: бот сам пишет пользователю «Вы пытаетесь подключить аккаунт…».
-- telegram_id считается подтверждённым только вместе с telegram_verified_at.
ALTER TABLE users ADD COLUMN telegram_verified_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN telegram_prompt_at TIMESTAMPTZ;
UPDATE users SET telegram_verified_at = now() WHERE telegram_id IS NOT NULL;

-- Кто уже писал боту: по @имени знаем, в какой чат отправить запрос на подтверждение
CREATE TABLE telegram_contacts (
    username   VARCHAR(64) PRIMARY KEY,
    tg_id      BIGINT NOT NULL,
    chat_id    BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
