FROM golang:1.25-alpine AS build
WORKDIR /src
# Зеркала Go-модулей: если proxy.golang.org (storage.googleapis.com) недоступен — берём из зеркал.
# Можно переопределить: docker compose build --build-arg GOPROXY=...
ARG GOPROXY=https://goproxy.cn,https://goproxy.io,https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY GOSUMDB=off GOFLAGS=-mod=mod
COPY go.mod go.sum* ./
COPY . .
# Кеш модулей сохраняется между сборками: при обрыве сети повтор докачивает, а не начинает заново.
RUN --mount=type=cache,id=gomod,target=/go/pkg/mod \
    --mount=type=cache,id=gobuild,target=/root/.cache/go-build \
    for i in 1 2 3 4 5 6; do go mod tidy && break; echo "go mod tidy: попытка $i не удалась, повтор"; sleep 5; [ "$i" = 6 ] && exit 1; done && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

FROM alpine:3.20
ARG APK_MIRRORS="https://dl-cdn.alpinelinux.org/alpine https://mirror.yandex.ru/mirrors/alpine https://mirrors.aliyun.com/alpine https://ftp.halifax.rwth-aachen.de/alpine"
# Установка пакетов с повторами и запасными зеркалами: при сбое сети сборка не падает сразу.
# tesseract (OCR, рус+англ) и poppler (текст и картинка из PDF) — для проверки чеков.
# Основа (tzdata, ca-certificates) обязательна, а OCR-пакеты — нет: без них чеки читаются по QR и тексту PDF,
# а фото администратор проверяет вручную (в Настройках видно, какие способы доступны).
RUN set -e; \
    apkget() { \
      for m in $APK_MIRRORS; do \
        for i in 1 2 3; do \
          printf '%s/v3.20/main\n%s/v3.20/community\n' "$m" "$m" > /etc/apk/repositories; \
          if apk add --no-cache "$@"; then return 0; fi; \
          echo "apk: зеркало $m, попытка $i не удалась"; sleep 3; \
        done; \
      done; return 1; \
    }; \
    apkget tzdata ca-certificates; \
    apkget tesseract-ocr tesseract-ocr-data-rus tesseract-ocr-data-eng poppler-utils \
      || echo "ВНИМАНИЕ: OCR-пакеты не установились, проверка чеков будет работать без OCR"
WORKDIR /app
COPY --from=build /out/server /out/bot /app/
# не от root: при уязвимости в приложении у атакующего не будет прав администратора в контейнере
RUN adduser -D -H -u 10001 app
USER app
EXPOSE 8080
# По умолчанию — API. Бот запускается тем же образом с командой /app/bot (см. docker-compose.yml)
CMD ["/app/server"]
