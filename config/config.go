package config

import (
	"fmt"
	"time"

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
		SecOptions: secure.Options{
			BrowserXssFilter:   true,
			ContentTypeNosniff: true,
			SSLHost:            "locahost:443",
			SSLRedirect:        false,
		},
	}

	return nil
}
