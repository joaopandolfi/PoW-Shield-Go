package handler

import (
	"fmt"
	"net/http"
	"pow-shield-go/models/domain"
	"time"
)

const TOKEN_COOKIE = "powShield"

func SetCookie(w http.ResponseWriter, cookie *domain.Cookie) {
	expires := time.Now().AddDate(1, 0, 0)
	http.SetCookie(w, &http.Cookie{
		Name:     TOKEN_COOKIE,
		Value:    cookie.Value,
		Path:     cookie.Path,
		MaxAge:   cookie.MaxAge,
		HttpOnly: cookie.HttpOnly,
		Secure:   cookie.Secure,
		Expires:  expires,
		SameSite: http.SameSiteLaxMode,
		//SameSite: http.SameSiteDefaultMode,
	})
}

func GetCookie(r *http.Request) (*domain.Cookie, error) {
	netCookie, err := r.Cookie(TOKEN_COOKIE)
	if err != nil {
		return nil, fmt.Errorf("reading cookie: %w", err)
	}
	return &domain.Cookie{
		Value:   netCookie.Value,
		Path:    netCookie.Path,
		Expires: netCookie.Expires,
		MaxAge:  netCookie.MaxAge,
	}, nil
}

func CleanCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   TOKEN_COOKIE,
		MaxAge: -1,
	})
}
