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

	ProtectedServer protectedServer

	Pow     pow
	Session session
	Static  static
}

type static struct {
	ServeStatic  bool
	StaticPath   string
	StaticFolder string
}

type protectedServer struct {
	Host           string
	DefaultHeaders map[string]string
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
			Name:  "token-session",
			Store: sessions.NewCookieStore([]byte("SESSION_STORE")),
			Options: &sessions.Options{
				Path:     "/",
				MaxAge:   3600 * 2, //86400 * 7,
				HttpOnly: true,
				Secure:   true,
			},
		},
		Pow: pow{
			DefaultPrefixSize: 15,
			NonceValidity:     150 * 1000, //miliseconds -> senconds
		},
		ProtectedServer: protectedServer{
			Host:           "http://localhost:3002",
			DefaultHeaders: map[string]string{},
		},
		Static: static{
			ServeStatic:  true,
			StaticPath:   "/public/",
			StaticFolder: "../client/public/",
		},
	}

	return nil
}
