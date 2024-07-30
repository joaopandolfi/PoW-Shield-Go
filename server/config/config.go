package config

import (
	"fmt"
	"time"

	"github.com/gorilla/sessions"
	"github.com/unrolled/secure"
)

type Config struct {
	Port         string
	WriteTimeout time.Duration
	ReadTimeout  time.Duration
	UseTLS       bool
	TLSCert      string
	TLSKey       string
	SecOptions   secure.Options
	Cors         cors
	Cache        cache

	Pow     pow
	Session session
}

type pow struct {
	NonceValidity     int
	DefaultPrefixSize int
}

type cache struct {
	Redis redis
}

type redis struct {
	Use      bool
	Server   string
	Password string
	DB       int
}

type cors struct {
	Active   bool
	Enableds string
}

type session struct {
	Name    string
	Store   *sessions.CookieStore
	Options *sessions.Options
}

var cfg *Config

func Get() Config {
	if cfg == nil {
		panic(fmt.Errorf("config not loaded"))
	}

	return *cfg
}

func Inject(c Config) {
	cfg = &c
}

func Load() error {

	if cfg != nil {
		return nil
	}

	cfg = &Config{
		Port: ":5656",
		SecOptions: secure.Options{
			BrowserXssFilter:   true,
			ContentTypeNosniff: true,
			SSLHost:            "locahost:443",
			SSLRedirect:        false,
		},
		Session: session{
			Name:  "SESSION_NAME",
			Store: sessions.NewCookieStore([]byte("SESSION_STORE")),
			Options: &sessions.Options{
				Path:     "/",
				MaxAge:   3600 * 2, //86400 * 7,
				HttpOnly: true,
			},
		},
		Pow: pow{
			DefaultPrefixSize: 16,
			NonceValidity:     15 * 1000, //miliseconds
		},
	}

	return nil
}
