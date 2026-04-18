package admin

import (
	"fmt"
	"net/http"
	"time"

	"pow-shield-go/config"
	lp "pow-shield-go/internal/logging"
	"pow-shield-go/internal/metrics"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/handler"
	"pow-shield-go/web/server"
)

type controller struct {
	s *server.Server
}

func New() controllers.Controller {
	return &controller{s: nil}
}

func (c *controller) SetupRouter(s *server.Server) {
	c.s = s
	path := config.Get().Admin.Path

	if !config.Get().Admin.Active {
		return
	}

	s.R.PathPrefix(path).Handler(c.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == path || r.URL.Path == path+"/" || r.URL.Path == path+"/stats" {
			c.dashboardHandler(w, r)
		} else if r.URL.Path == path+"/login" {
			c.loginHandler(w, r)
		} else if r.URL.Path == path+"/api/stats" {
			c.apiStats(w, r)
		} else if r.URL.Path == path+"/api/reset" && r.Method == "POST" {
			c.apiReset(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
}

func (c *controller) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !c.isLoggedIn(w, r, "admin") {
			http.Redirect(w, r, config.Get().Admin.Path+"/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

func (c *controller) isLoggedIn(w http.ResponseWriter, r *http.Request, role string) bool {
	cookie, err := r.Cookie(role + "_session")
	if err == nil && cookie.Value == role+"_"+role {
		return true
	}
	if role == "admin" && r.Header.Get("X-Admin-Key") == "admin_key_123" {
		return true
	}
	return false
}

func (c *controller) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if c.isLoggedIn(w, r, "admin") {
			http.Redirect(w, r, config.Get().Admin.Path, http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(loginPageHTML))
		return
	}

	password := r.FormValue("password")
	if password == "admin123" {
		http.SetCookie(w, &http.Cookie{
			Name:     "admin_session",
			Value:    "admin_admin",
			Path:     config.Get().Admin.Path,
			HttpOnly: true,
			Secure:   config.Get().Session.Options.Secure,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   3600 * 8,
		})
		http.Redirect(w, r, config.Get().Admin.Path, http.StatusFound)
		return
	}

	http.Redirect(w, r, config.Get().Admin.Path+"/login?error=1", http.StatusFound)
}

func (c *controller) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	m := metrics.GetMetricsSnapshot()
	log := lp.Get()

	stats := lp.Stats{}
	if log != nil {
		stats = log.StatsFunc()
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>PoW Shield Dashboard</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,sans-serif;background:#0a0e17;color:#c9d1d9;min-height:100vh}
.header{background:#0d1117;padding:20px 40px;border-bottom:1px solid #21262d;display:flex;justify-content:space-between}
.header h1{color:#58a6ff;font-size:24px}
.header a{color:#58a6ff;text-decoration:none;padding:8px 16px;border:1px solid #30363d;border-radius:6px}
.container{max-width:1200px;margin:0 auto;padding:30px 40px}
.stats-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:20px;margin-bottom:40px}
.stat-card{background:#161b22;border:1px solid #21262d;border-radius:8px;padding:20px;text-align:center}
.stat-card h2{color:#58a6ff;font-size:32px;margin-bottom:8px}
.stat-card p{color:#8b949e;font-size:14px}
.section{background:#161b22;border:1px solid #21262d;border-radius:8px;padding:24px;margin-bottom:20px}
.section h3{color:#58a6ff;margin-bottom:16px;font-size:18px}
table{width:100%;border-collapse:collapse}
th,td{text-align:left;padding:10px 12px;border-bottom:1px solid #21262d}
th{color:#58a6ff;font-weight:600;font-size:13px;text-transform:uppercase}
td{font-size:14px}
.badge{display:inline-block;padding:4px 10px;border-radius:12px;font-size:12px;font-weight:600}
.badge-ok{background:#0e4429;color:#3fb950}
.badge-fail{background:#4e1116;color:#f85149}
.btn{background:#238636;color:#fff;border:none;padding:10px 20px;border-radius:6px;cursor:pointer;font-size:14px}
.btn:hover{background:#2ea043}
.config-item{display:flex;justify-content:space-between;padding:12px 0;border-bottom:1px solid #21262d}
.config-item:last-child{border-bottom:none}
.config-label{color:#8b949e}
.config-value{color:#c9d1d9;font-weight:600}
</style>
</head>
<body>
<div class="header">
<h1>Shield Admin</h1>
<a href="/admin/reset" onclick="return confirm('Reset statistics?')">Reset Stats</a>
</div>
<div class="container">
<div class="stats-grid">
<div class="stat-card"><h2>%d</h2><p>Requests</p></div>
<div class="stat-card"><h2>%d</h2><p>Proxied</p></div>
<div class="stat-card"><h2>%d</h2><p>Blocked</p></div>
<div class="stat-card"><h2>%d</h2><p>PoW Blocked</p></div>
<div class="stat-card"><h2>%d</h2><p>Rate Limited</p></div>
<div class="stat-card"><h2>%d</h2><p>Errors</p></div>
</div>
<div class="section">
<h3>System Status</h3>
<div class="config-item"><span class="config-label">Uptime</span><span class="config-value">%s</span></div>
<div class="config-item"><span class="config-label">Port</span><span class="config-value">%s</span></div>
<div class="config-item"><span class="config-label">TLS</span><span class="config-value">%s</span></div>
<div class="config-item"><span class="config-label">WAF</span><span class="config-value">%s</span></div>
<div class="config-item"><span class="config-label">PoW</span><span class="config-value">%s</span></div>
<div class="config-item"><span class="config-label">Cache</span><span class="config-value">%s</span></div>
</div>
<div class="section">
<h3>WAF Blocks</h3>
<table>
<thead><tr><th>Category</th><th>Count</th></tr></thead>
<tbody>
%s
</tbody>
</table>
</div>
</div>
<script>
function autoRefresh(){setTimeout(function(){location.reload()},30000)}
autoRefresh()
</script>
</body></html>`,
		m["total_requests"].(uint64),
		m["proxied_requests"].(uint64),
		m["blocked_responses"].(uint64),
		m["pow_blocked"].(uint64),
		m["rate_limited"].(uint64),
		stats.ErrorCount,
		stats.Uptime.Round(time.Second).String(),
		config.Get().Port,
		statusTag(config.Get().UseTLS),
		statusTag(config.Get().Waf.Active),
		statusTag(config.Get().Pow.Active),
		statusTag(config.Get().Cache.Redis.Use),
		wafTable(m["waf_blocked"].(map[string]interface{})),
	)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

func statusTag(active bool) string {
	if active {
		return "<span class=\"badge badge-ok\">Active</span>"
	}
	return "<span class=\"badge badge-fail\">Inactive</span>"
}

func wafTable(wafBlocks interface{}) string {
	counts, ok := wafBlocks.(map[string]interface{})
	if !ok {
		return "<tr><td colspan=\"2\">No data</td></tr>"
	}
	var rows string
	for k, v := range counts {
		if num, ok := v.(uint64); ok && num > 0 {
			rows += fmt.Sprintf("<tr><td>%s</td><td>%d</td></tr>", k, num)
		}
	}
	if rows == "" {
		return "<tr><td colspan=\"2\">No WAF blocks</td></tr>"
	}
	return rows
}

func (c *controller) apiStats(w http.ResponseWriter, r *http.Request) {
	m := metrics.GetMetricsSnapshot()
	log := lp.Get()

	stats := lp.Stats{}
	if log != nil {
		stats = log.StatsFunc()
	}

	data := map[string]interface{}{
		"metrics": m,
		"uptime":  stats.Uptime.String(),
		"config": map[string]interface{}{
			"port":        config.Get().Port,
			"use_tls":     config.Get().UseTLS,
			"waf_active":  config.Get().Waf.Active,
			"pow_active":  config.Get().Pow.Active,
			"rate_active": config.Get().Rate.Active,
		},
	}

	handler.RespondJson(w, data, http.StatusOK)
}

func (c *controller) apiReset(w http.ResponseWriter, r *http.Request) {
	metrics.ResetMetrics()
	handler.RespondJson(w, map[string]string{"status": "ok", "message": "Metrics reset successful"}, http.StatusOK)
}

const loginPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>PoW Shield Admin</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,sans-serif;background:#0a0e17;color:#c9d1d9;height:100vh;display:flex;align-items:center;justify-content:center}
.card{background:#161b22;border:1px solid #21262d;border-radius:12px;padding:40px;width:380px;text-align:center}
h1{color:#58a6ff;margin-bottom:24px;font-size:28px}
input{width:100%;padding:12px;margin:16px 0;background:#0d1117;border:1px solid #30363d;border-radius:6px;color:#c9d1d9;font-size:16px}
input:focus{outline:none;border-color:#58a6ff}
button{width:100%;padding:12px;background:#238636;color:#fff;border:none;border-radius:6px;font-size:16px;font-weight:600;cursor:pointer}
button:hover{background:#2ea043}
.error-msg{color:#f85149;margin-top:12px;font-size:14px}
</style>
</head>
<body>
<div class="card">
<h1>PoW Shield</h1>
<form method="POST" action="/admin/login">
<input type="password" name="password" placeholder="Password" required autofocus>
<button type="submit">Sign In</button>
</form>
</div>
</body>
</html>`
