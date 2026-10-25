-- Привязка Telegram по @имени пользователя (вместо числового ID). telegram_id остаётся и
-- заполняется сам, когда человека удалось опознать в боте.
ALTER TABLE users ADD COLUMN telegram_username VARCHAR(64);
CREATE UNIQUE INDEX uq_users_telegram_username ON users (lower(telegram_username)) WHERE telegram_username IS NOT NULL;
