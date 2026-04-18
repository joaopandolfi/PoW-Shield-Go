package admin

import (
	"crypto/rand"
	"embed"
	"encoding/json"
	"net/http"

	"pow-shield-go/config"
	lp "pow-shield-go/internal/logging"
	"pow-shield-go/internal/metrics"
	"pow-shield-go/web/controllers"
	"pow-shield-go/web/handler"
	"pow-shield-go/web/server"
)

//go:embed static/admin/*
var adminFiles embed.FS

type controller struct {
	s *server.Server
}

type loginPayload struct {
	Password string `json:"password"`
	Error    string `json:"error,omitempty"`
	Ok       bool   `json:"ok,omitempty"`
}

func New() controllers.Controller {
	return &controller{s: nil}
}

func (c *controller) SetupRouter(s *server.Server) {
	c.s = s
	if !config.Get().Admin.Active {
		return
	}

	path := config.Get().Admin.Path

	s.R.HandleFunc(path, c.requireAuth(c.indexHandler)).Methods("GET", "HEAD")
	s.R.HandleFunc(path+"/login", c.loginHandler).Methods("GET", "HEAD")
	s.R.HandleFunc(path+"/api/login", c.loginPostHandler).Methods("POST")
	s.R.HandleFunc(path+"/api/logout", c.requireAuth(c.logoutHandler)).Methods("POST", "GET", "HEAD")
	s.R.HandleFunc(path+"/api/stats", c.requireAuth(c.apiStats)).Methods("GET", "HEAD")
	s.R.HandleFunc(path+"/api/reset", c.requireAuth(c.apiReset)).Methods("POST")
	s.R.HandleFunc(path+"/api/check", c.apiCheck).Methods("GET")
	s.R.PathPrefix(path + "/static/").HandlerFunc(c.spaStaticHandler)
	s.R.HandleFunc(path+"/{filename}", c.requireAuth(c.spaFileHandler)).Methods("GET")
}

func (c *controller) isLoggedIn(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie("admin_session")
	if err == nil && cookie.Value == "admin" {
		return true
	}
	if config.Get().Admin.Key != "" && r.Header.Get("X-Admin-Key") == config.Get().Admin.Key {
		return true
	}
	return false
}

func (c *controller) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !c.isLoggedIn(w, r) {
			if r.Header.Get("Accept") == "application/json" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			} else {
				http.Redirect(w, r, config.Get().Admin.Path+"/login", http.StatusFound)
			}
		} else {
			next(w, r)
		}
	}
}

func (c *controller) indexHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, config.Get().Admin.Path+"/dashboard.html", http.StatusFound)
}

func (c *controller) spaStaticHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len(config.Get().Admin.Path+"/static/"):]
	filePath := "client/public" + path
	if _, err := http.Dir(filePath).Open("."); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filePath)
}

func (c *controller) spaFileHandler(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" {
		http.NotFound(w, r)
		return
	}

	filePath := "admin/" + filename
	data, err := adminFiles.ReadFile(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (c *controller) loginHandler(w http.ResponseWriter, r *http.Request) {
	if c.isLoggedIn(w, r) {
		http.Redirect(w, r, config.Get().Admin.Path+"/dashboard.html", http.StatusFound)
		return
	}
	data, err := adminFiles.ReadFile("admin/login.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (c *controller) loginPostHandler(w http.ResponseWriter, r *http.Request) {
	var data loginPayload
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	expectedPassword := config.Get().Admin.Password
	if expectedPassword == "" {
		expectedPassword = "admin123"
	}

	if data.Password == expectedPassword {
		token := make([]byte, 16)
		rand.Read(token)
		http.SetCookie(w, &http.Cookie{
			Name:     "admin_session",
			Value:    "admin",
			Path:     config.Get().Admin.Path,
			HttpOnly: true,
			Secure:   config.Get().Session.Options.Secure,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   3600 * 8,
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
}

func (c *controller) logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "admin",
		Path:     config.Get().Admin.Path,
		HttpOnly: true,
		Secure:   config.Get().Session.Options.Secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
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
		"errors": int(stats.ErrorCount),
	}

	handler.RespondJson(w, data, http.StatusOK)
}

func (c *controller) apiReset(w http.ResponseWriter, r *http.Request) {
	metrics.ResetMetrics()
	handler.RespondJson(w, map[string]string{"status": "ok", "message": "Metrics reset successful"}, http.StatusOK)
}

func (c *controller) apiCheck(w http.ResponseWriter, r *http.Request) {
	if c.isLoggedIn(w, r) {
		handler.RespondJson(w, map[string]interface{}{"ok": true, "auth": "cookie"}, http.StatusOK)
	} else if key := r.Header.Get("X-Admin-Key"); key == config.Get().Admin.Key {
		handler.RespondJson(w, map[string]interface{}{"ok": true, "auth": "header"}, http.StatusOK)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false})
	}
}
