#!/usr/bin/env bash
# Деплой Invoice Keeper на чистый Ubuntu-сервер: Docker + .env с случайными секретами + запуск на порту 80.
# Запуск (из папки проекта):  TELEGRAM_BOT_TOKEN=123:abc bash deploy/server-setup.sh
set -euo pipefail
cd "$(dirname "$0")/.."

if ! command -v docker >/dev/null 2>&1; then
  echo ">> Устанавливаю Docker"
  apt-get update -y && apt-get install -y ca-certificates curl
  curl -fsSL https://get.docker.com | sh
fi
docker compose version >/dev/null

IP=$(curl -fsS4 https://ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')

if [ ! -f .env ]; then
  echo ">> Создаю .env со случайными секретами"
  ADMIN_PASSWORD=$(openssl rand -base64 12 | tr -d '/+=' | head -c 14)
  cat > .env <<ENV
ADMIN_LOGIN=admin
ADMIN_PASSWORD=${ADMIN_PASSWORD}
JWT_SECRET=$(openssl rand -hex 32)
TZ=Europe/Moscow
TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN:-}
TELEGRAM_CHAT_ID=
REMINDER_STAGES=7,3,1
CHECK_INTERVAL=1m
ALLOW_REGISTRATION=true
SEED_DEMO=true
DEMO_MODE=true
POSTGRES_USER=invoice
POSTGRES_PASSWORD=$(openssl rand -hex 16)
POSTGRES_DB=invoice
FRONTEND_PORT=80
BACKEND_PORT=18080
PUBLIC_URL=http://${IP}
ENV
  chmod 600 .env
  echo ">> Пароль администратора: ${ADMIN_PASSWORD}  (логин admin; он же сохранён в .env)"
else
  echo ">> .env уже есть — меняю только порты и адрес сайта под сервер"
  sed -i -e 's/^FRONTEND_PORT=.*/FRONTEND_PORT=80/' -e 's/^BACKEND_PORT=.*/BACKEND_PORT=18080/' -e "s#^PUBLIC_URL=.*#PUBLIC_URL=http://${IP}#" .env
  # тестовая версия для тестеров: на сайте появляются образцы чеков
  if grep -q '^DEMO_MODE=' .env; then sed -i 's/^DEMO_MODE=.*/DEMO_MODE=true/' .env; else echo 'DEMO_MODE=true' >> .env; fi
  grep -q '^TELEGRAM_BOT_TOKEN=.' .env || { [ -n "${TELEGRAM_BOT_TOKEN:-}" ] && sed -i "s#^TELEGRAM_BOT_TOKEN=.*#TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}#" .env; }
  echo ">> Логин и пароль администратора — из вашего .env (ADMIN_LOGIN / ADMIN_PASSWORD)"
fi

echo ">> Собираю и запускаю (первая сборка 3-6 минут)"
docker compose up -d --build

echo ">> Жду, пока API ответит"
for i in $(seq 1 60); do
  curl -fsS http://127.0.0.1:80/api/v1/health >/dev/null 2>&1 && break || sleep 3
done
docker compose ps
echo
echo "Готово: http://${IP}/    Swagger: http://${IP}/api/docs (или /api/v1/docs)"
