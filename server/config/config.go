package config

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
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
	Waf     waf
}

type waf struct {
	Active               bool
	WhiteListURLRules    []int
	WhiteListHeaderRules []int
	WhiteListBodyRules   []int
	RawWafs              string
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
	Punishment        int
	IPTollerance      int
	UseCookie         bool
	UseSession        bool
	UseHeader         bool
	Active            bool
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
	Store   *sessions.CookieStore //*sessions.FilesystemStore
	Fstore  *sessions.FilesystemStore
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
	godotenv.Load(".env")

	cfg = &Config{
		Port:    fmt.Sprintf(":%s", getEnvOrDefault("PORT", "5656")),
		UseTLS:  StrTo[bool](getEnvOrDefault("USE_TLS", "false")),
		TLSCert: getEnvOrDefault("TLS_CERT", ""),
		TLSKey:  getEnvOrDefault("TLS_KEY", ""),

		SecOptions: secure.Options{
			BrowserXssFilter:   StrTo[bool](getEnvOrDefault("SEC_BROWSER_XSS_FILTER", "true")),
			ContentTypeNosniff: StrTo[bool](getEnvOrDefault("SEC_CONTENT_TYPE_NO_SNIFF", "true")),
			SSLHost:            getEnvOrDefault("SEC_SSL_HOST", "localhost:443"),
			SSLRedirect:        StrTo[bool](getEnvOrDefault("SEC_SSL_REDIRECT", "false")),
		},
		Session: session{
			Name:  getEnvOrDefault("SESSION_NAME", "PoW-Session"),
			Store: sessions.NewCookieStore([]byte(getEnvOrDefault("SESSION_PASS", "12345670101112ABC"))),
			Options: &sessions.Options{
				Path:     getEnvOrDefault("SESSION_PATH", "/"),
				MaxAge:   StrTo[int](getEnvOrDefault("SESSION_MAX_AGE", "7200")), //3600 * 2, //86400 * 7,
				HttpOnly: StrTo[bool](getEnvOrDefault("SESSION_HTTP_ONLY", "true")),
				Secure:   StrTo[bool](getEnvOrDefault("SESSION_SECURE", "true")),
			},
		},
		Pow: pow{
			DefaultPrefixSize: StrTo[int](getEnvOrDefault("DEFAULT_PREFIX_SIZE", "15")),
			Punishment:        StrTo[int](getEnvOrDefault("PUNISHMENT", "1")),
			IPTollerance:      StrTo[int](getEnvOrDefault("IP_TOLLERANCE", "1")),
			NonceValidity:     StrTo[int](getEnvOrDefault("NONCE_VALIDITY", "150000")), // 150 * 1000, //miliseconds -> senconds
			UseCookie:         StrTo[bool](getEnvOrDefault("USE_COOKIE", "true")),
			UseSession:        StrTo[bool](getEnvOrDefault("USE_SESSION", "true")),
			UseHeader:         StrTo[bool](getEnvOrDefault("USE_HEADER", "true")),
			Active:            StrTo[bool](getEnvOrDefault("POW_ACTIVE", "true")),
		},
		ProtectedServer: protectedServer{
			Host:           getEnvOrDefault("PROTECTED_SERVER_HEADERS", "http://localhost:3001"),
			DefaultHeaders: StrTo[map[string]string](getEnvOrDefault("PROTECTED_SERVER_HEADERS", "[]")),
		},
		Static: static{
			ServeStatic:  StrTo[bool](getEnvOrDefault("SERVE_STATIC", "true")),
			StaticPath:   getEnvOrDefault("SERVE_STATIC_PATH", "/public"),
			StaticFolder: getEnvOrDefault("SERVE_STATIC_FOLDER", "../client/public/"),
		},
		Waf: waf{
			Active:               StrTo[bool](getEnvOrDefault("WAF_ACTIVE", "true")),
			WhiteListURLRules:    StrTo[[]int](getEnvOrDefault("WHITELIST_URL_RULES", "[]")),
			WhiteListBodyRules:   StrTo[[]int](getEnvOrDefault("WHITELIST_BODY_RULES", "[]")),
			WhiteListHeaderRules: StrTo[[]int](getEnvOrDefault("WHITELIST_HEADER_RULES", "[]")),
			RawWafs:              loadWafRules(getEnvOrDefault("WAF_RULES_FILE", "wafRules.json")),
		},
		Cache: cache{
			Redis: redis{
				Use:      StrTo[bool](getEnvOrDefault("REDIS_USE", "false")),
				DB:       StrTo[int](getEnvOrDefault("REDIS_DB", "1")),
				Server:   getEnvOrDefault("REDIS_HOST", "localhost:6379"),
				Password: getEnvOrDefault("REDIS_PASS", ""),
			},
		},
	}

	if !cfg.Pow.UseCookie && !cfg.Pow.UseSession && !cfg.Pow.UseHeader && cfg.Pow.Active {
		return fmt.Errorf("to use PoW protection you need to setup USE_COOKIE,USE_SESSION or USE_HEADER (both can be true)")
	}

	return nil
}

func loadWafRules(fileName string) string {
	rules := "[]"
	file, err := os.Open(fileName)
	if err != nil {
		log.Println("[!][Config] Loading waf rule file: ", err.Error())
		return rules
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		log.Println("[!][Config] Rading waf rule file: ", err.Error())
		return rules
	}
	return string(b)
}
