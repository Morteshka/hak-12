// Cloudflare Worker: зеркало Telegram Bot API для серверов, у которых api.telegram.org не открывается.
// Бесплатно: dash.cloudflare.com → Workers & Pages → Create → Hello World → Edit code → вставить этот файл → Deploy.
// Затем в .env на сервере: TELEGRAM_API_ENDPOINT=https://<имя>.<аккаунт>.workers.dev/bot%s/%s
export default {
  async fetch(request) {
    const url = new URL(request.url)
    // пропускаем только запросы Bot API и скачивание файлов
    if (!/^\/(file\/)?bot[^/]+\//.test(url.pathname)) return new Response('not found', { status: 404 })
    url.protocol = 'https:'
    url.hostname = 'api.telegram.org'
    url.port = ''
    return fetch(new Request(url, request))
  },
}
