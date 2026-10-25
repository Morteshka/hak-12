package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"invoicekeeper/api"
	"invoicekeeper/internal/models"
)

// DocsCookie — токен администратора для Swagger. Кука живёт только на пути /api/v1/docs
// (SameSite=Strict), поэтому для остального API по-прежнему нужен заголовок Authorization.
const DocsCookie = "ik_docs"

// docsAccess пускает к документации только администратора: токен берётся из заголовка или куки ik_docs.
// Возвращает 0, если доступ есть, иначе HTTP-код (401 — не вошли, 403 — не администратор).
func (h *handlers) docsAccess(c *gin.Context) int {
	tok, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || tok == "" {
		tok, _ = c.Cookie(DocsCookie)
	}
	if tok == "" {
		return http.StatusUnauthorized
	}
	u, _, err := h.Auth.Authenticate(c.Request.Context(), tok)
	if err != nil {
		return http.StatusUnauthorized
	}
	if u.Role != models.RoleAdmin {
		return http.StatusForbidden
	}
	return 0
}

// docsPage — Swagger UI для администратора; остальным — форма входа администратора.
func (h *handlers) docsPage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	switch h.docsAccess(c) {
	case 0:
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML))
	case http.StatusForbidden:
		c.Data(http.StatusForbidden, "text/html; charset=utf-8", []byte(strings.Replace(docsLoginHTML, "{{msg}}",
			"Вы вошли как обычный пользователь. Документация API доступна только администратору.", 1)))
	default:
		c.Data(http.StatusUnauthorized, "text/html; charset=utf-8", []byte(strings.Replace(docsLoginHTML, "{{msg}}", "", 1)))
	}
}

// docsSpec — openapi.yml, тоже только администратору.
func (h *handlers) docsSpec(c *gin.Context) {
	if code := h.docsAccess(c); code != 0 {
		msg := map[int]string{http.StatusUnauthorized: "authorization required", http.StatusForbidden: "forbidden: admin role required"}[code]
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.Data(http.StatusOK, "application/yaml", api.OpenAPI)
}

// Swagger UI: токен из куки подставляется в «Try it out» автоматически — нажимать Authorize не нужно.
const swaggerHTML = `<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Invoice Keeper API</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css">
<style>.ik-bar{display:flex;gap:14px;align-items:center;justify-content:flex-end;padding:10px 20px;font:14px system-ui,sans-serif;background:#16181d;color:#fff}
.ik-bar a{color:#fff}</style></head>
<body><div class="ik-bar"><span>Документация для администратора</span><a href="/">На сайт</a><a href="#" onclick="logout();return false">Выйти</a></div><div id="ui"></div>
<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
<script>
function token(){var m=document.cookie.match(/(?:^|; )ik_docs=([^;]+)/);return m?decodeURIComponent(m[1]):""}
function logout(){document.cookie="ik_docs=; path=/api/v1/docs; max-age=0; SameSite=Strict";location.reload()}
window.ui = SwaggerUIBundle({
  url: "/api/v1/docs/openapi.yml", dom_id: "#ui",
  requestInterceptor: function(r){var t=token(); if(t && !r.headers.Authorization){r.headers.Authorization="Bearer "+t} return r}
});
</script>
</body></html>`

// Форма входа: логин/пароль → /auth/login; если роль admin — ставим куку и перезагружаем страницу.
const docsLoginHTML = `<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Invoice Keeper API — вход</title>
<style>
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:#f4f1ec;font:15px system-ui,-apple-system,Segoe UI,Roboto,sans-serif;color:#16181d;padding:16px;box-sizing:border-box}
form{width:100%;max-width:380px;background:#fff;border-radius:24px;padding:28px;box-shadow:0 10px 40px rgba(0,0,0,.06)}
h1{font-size:22px;margin:0 0 6px}p{color:#6b6e76;margin:0 0 14px;line-height:1.45}
label{display:block;font-size:13px;color:#6b6e76;margin:12px 0 6px}
input{width:100%;box-sizing:border-box;height:46px;border:1px solid #e3e0da;border-radius:14px;padding:0 14px;font:inherit}
button{margin-top:18px;width:100%;height:48px;border:0;border-radius:999px;background:#16181d;color:#fff;font:600 15px system-ui,sans-serif;cursor:pointer}
button:disabled{opacity:.5}.err{color:#b42318;min-height:20px;font-size:14px}a{color:#16181d}
</style></head>
<body><form id="f">
<h1>Документация API</h1>
<p>Swagger доступен только администратору. Войдите с логином и паролем администратора (ADMIN_LOGIN / ADMIN_PASSWORD из .env).</p>
<div class="err" id="e">{{msg}}</div>
<label for="l">Логин</label><input id="l" autocomplete="username" required>
<label for="p">Пароль</label><input id="p" type="password" autocomplete="current-password" required>
<button id="b">Войти</button>
<p style="margin:16px 0 0;font-size:13px"><a href="/">← На сайт</a></p>
</form>
<script>
var f=document.getElementById("f"),e=document.getElementById("e"),b=document.getElementById("b");
f.onsubmit=async function(ev){ev.preventDefault();b.disabled=true;e.textContent="";
 try{
  var r=await fetch("/api/v1/auth/login",{method:"POST",headers:{"Content-Type":"application/json"},
    body:JSON.stringify({login:document.getElementById("l").value.trim(),password:document.getElementById("p").value})});
  var d=await r.json();
  if(!r.ok){e.textContent=r.status===401?"Неверный логин или пароль":(d.error||"Ошибка входа");return}
  if(!d.user||d.user.role!=="admin"){e.textContent="Документация доступна только администратору.";
    fetch("/api/v1/auth/logout",{method:"POST",headers:{Authorization:"Bearer "+d.access_token}});return}
  document.cookie="ik_docs="+encodeURIComponent(d.access_token)+"; path=/api/v1/docs; max-age="+(d.expires_in||900)+"; SameSite=Strict";
  location.reload();
 }catch(x){e.textContent="Сервер недоступен"}finally{b.disabled=false}
};
</script>
</body></html>`
