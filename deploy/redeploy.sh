#!/bin/sh
# Обновление сайта одной командой: sh /root/invoice-keeper/deploy/redeploy.sh
# Распаковывает /root/ik.tgz в /root/invoice-keeper, пересобирает контейнеры и проверяет, что всё поднялось.
set -e
cd /root/invoice-keeper
[ -f /root/ik.tgz ] || { echo "Нет /root/ik.tgz — сначала загрузите архив через scp"; exit 1; }
echo "== Распаковка архива"
tar -xzf /root/ik.tgz
echo "== В архиве версия: $(cat frontend/public/version.txt)"
echo "== Сборка (если тут ошибка — пришлите её текст)"
docker compose build frontend backend bot
echo "== Перезапуск"
docker compose up -d --force-recreate
sleep 6
PORT=$(grep -E '^FRONTEND_PORT=' .env 2>/dev/null | cut -d= -f2); PORT=${PORT:-3000}
echo "== Проверка nginx: $(docker compose exec -T frontend nginx -t 2>&1 | tail -1)"
echo "== Сайт отвечает версией: $(curl -s http://localhost:$PORT/version.txt)"
echo "== API: $(curl -s http://localhost:$PORT/api/v1/health)"
docker compose logs backend 2>/dev/null | grep -E "ВНИМАНИЕ" | tail -3 || true
docker compose logs bot --tail 3 2>/dev/null | sed 's/^/   бот: /' || true
docker compose ps
