-- Discord убран из проекта: чистим его настройку и отметки доставки.
DELETE FROM app_settings WHERE key = 'discord_webhook_url';
DELETE FROM notification_deliveries WHERE channel = 'discord';
