package config

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	Metrics      metrics
	Admin        admin
	Logging      logging

	ProtectedServer protectedServer

	Pow     pow
	Rate    rateLimit
	Reply   responseConfig
	Session session
	Static  static
	Waf     waf
}

type logging struct {
	Level       string
	FilePath    string
	Stacktrace  bool
	Environment string
}

type admin struct {
	Active   bool
	Path     string
	Key      string
	Password string
}

type waf struct {
	Active               bool
	WhiteListURLRules    []int
	WhiteListHeaderRules []int
	WhiteListBodyRules   []int
	AllowTypes           []string
	BlockTypes           []string
	RawWafs              string
	RawWafTypes          string
}

type static struct {
	ServeStatic  bool
	StaticPath   string
	StaticFolder string
}

type protectedServer struct {
	Host               string
	DefaultHeaders     map[string]string
	Timeout            time.Duration
	InsecureSkipVerify bool
}

type pow struct {
	NonceValidity               int
	DefaultPrefixSize           int
	Punishment                  int
	IPTollerance                int
	IPTolleranceDurationSeconds int
	UseCookie                   bool
	UseSession                  bool
	UseHeader                   bool
	Active                      bool
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

type metrics struct {
	Active bool
	Path   string
}

type rateLimit struct {
	Active        bool
	Requests      int
	WindowSeconds int
}

type responseConfig struct {
	BlockStatus      int
	BlockBody        string
	BlockContentType string
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

func sessionPass() string {
	if value, ok := os.LookupEnv("SESSION_PASS"); ok && value != "" {
		return value
	}

	buff := make([]byte, 32)
	if _, err := rand.Read(buff); err != nil {
		fmt.Fprintf(os.Stderr, "[!][Config] failed to generate random SESSION_PASS, using development fallback\n")
		return "12345670101112ABC"
	}

	pass := fmt.Sprintf("%x", buff)
	fmt.Fprintf(os.Stderr, "[!][Config] SESSION_PASS not set, generated ephemeral random key\n")
	return pass
}

func Load() error {

	if cfg != nil {
		return nil
	}
	godotenv.Load(".env")
	sessionPassphrase := sessionPass()

	cfg = &Config{
		Port:         fmt.Sprintf(":%s", getEnvOrDefault("PORT", "5656")),
		WriteTimeout: time.Duration(StrTo[int](getEnvOrDefault("WRITE_TIMEOUT_SECONDS", "30"))) * time.Second,
		ReadTimeout:  time.Duration(StrTo[int](getEnvOrDefault("READ_TIMEOUT_SECONDS", "30"))) * time.Second,
		UseTLS:       StrTo[bool](getEnvOrDefault("USE_TLS", "false")),
		TLSCert:      getEnvOrDefault("TLS_CERT", ""),
		TLSKey:       getEnvOrDefault("TLS_KEY", ""),

		SecOptions: secure.Options{
			BrowserXssFilter:   StrTo[bool](getEnvOrDefault("SEC_BROWSER_XSS_FILTER", "true")),
			ContentTypeNosniff: StrTo[bool](getEnvOrDefault("SEC_CONTENT_TYPE_NO_SNIFF", "true")),
			SSLHost:            getEnvOrDefault("SEC_SSL_HOST", "localhost:443"),
			SSLRedirect:        StrTo[bool](getEnvOrDefault("SEC_SSL_REDIRECT", "false")),
		},
		Metrics: metrics{
			Active: StrTo[bool](getEnvOrDefault("METRICS_ACTIVE", "true")),
			Path:   getEnvOrDefault("METRICS_PATH", "/metrics"),
		},
		Session: session{
			Name:  getEnvOrDefault("SESSION_NAME", "PoW-Session"),
			Store: sessions.NewCookieStore([]byte(sessionPassphrase)),
			Fstore: sessions.NewFilesystemStore(
				filepath.Join(os.TempDir(), "pow-shield-go-sessions"),
				[]byte(sessionPassphrase),
			),
			Options: &sessions.Options{
				Path:     getEnvOrDefault("SESSION_PATH", "/"),
				MaxAge:   StrTo[int](getEnvOrDefault("SESSION_MAX_AGE", "7200")), //3600 * 2, //86400 * 7,
				HttpOnly: StrTo[bool](getEnvOrDefault("SESSION_HTTP_ONLY", "true")),
				Secure:   StrTo[bool](getEnvOrDefault("SESSION_SECURE", fmt.Sprintf("%t", StrTo[bool](getEnvOrDefault("USE_TLS", "false"))))),
			},
		},
		Pow: pow{
			DefaultPrefixSize:           StrTo[int](getEnvOrDefault("DEFAULT_PREFIX_SIZE", "15")),
			Punishment:                  StrTo[int](getEnvOrDefault("PUNISHMENT", "1")),
			IPTollerance:                StrTo[int](getEnvOrDefault("IP_TOLLERANCE", "1")),
			IPTolleranceDurationSeconds: StrTo[int](getEnvOrDefault("IP_TOLLERANCE_DURATION_SECONDS", "120")),
			NonceValidity:               StrTo[int](getEnvOrDefault("NONCE_VALIDITY", "150000")), // 150 * 1000, //miliseconds -> senconds
			UseCookie:                   StrTo[bool](getEnvOrDefault("USE_COOKIE", "true")),
			UseSession:                  StrTo[bool](getEnvOrDefault("USE_SESSION", "true")),
			UseHeader:                   StrTo[bool](getEnvOrDefault("USE_HEADER", "true")),
			Active:                      StrTo[bool](getEnvOrDefault("POW_ACTIVE", "true")),
		},
		ProtectedServer: protectedServer{
			Host:               getEnvOrDefault("PROTECTED_SERVER_HOST", "http://localhost:3001"),
			DefaultHeaders:     StrTo[map[string]string](getEnvOrDefault("PROTECTED_SERVER_HEADERS", "[]")),
			Timeout:            time.Duration(StrTo[int](getEnvOrDefault("PROTECTED_SERVER_TIMEOUT_SECONDS", "30"))) * time.Second,
			InsecureSkipVerify: StrTo[bool](getEnvOrDefault("PROTECTED_SERVER_INSECURE_SKIP_VERIFY", "false")),
		},
		Rate: rateLimit{
			Active:        StrTo[bool](getEnvOrDefault("RATE_LIMIT_ACTIVE", "true")),
			Requests:      StrTo[int](getEnvOrDefault("RATE_LIMIT_REQUESTS", "120")),
			WindowSeconds: StrTo[int](getEnvOrDefault("RATE_LIMIT_WINDOW_SECONDS", "60")),
		},
		Reply: responseConfig{
			BlockStatus:      StrTo[int](getEnvOrDefault("BLOCK_RESPONSE_STATUS", "406")),
			BlockBody:        getEnvOrDefault("BLOCK_RESPONSE_BODY", "blocked: x_x"),
			BlockContentType: getEnvOrDefault("BLOCK_RESPONSE_CONTENT_TYPE", "text/plain; charset=utf-8"),
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
			AllowTypes:           StrTo[[]string](getEnvOrDefault("WAF_ALLOW_TYPES", "[]")),
			BlockTypes:           StrTo[[]string](getEnvOrDefault("WAF_BLOCK_TYPES", "[]")),
			RawWafs:              loadWafRules(getEnvOrDefault("WAF_RULES_FILE", "wafRules.json")),
			RawWafTypes:          loadWafRules(getEnvOrDefault("WAF_TYPES_FILE", "wafTypes.json")),
		},
		Cache: cache{
			Redis: redis{
				Use:      StrTo[bool](getEnvOrDefault("REDIS_USE", "false")),
				DB:       StrTo[int](getEnvOrDefault("REDIS_DB", "1")),
				Server:   getEnvOrDefault("REDIS_HOST", "localhost:6379"),
				Password: getEnvOrDefault("REDIS_PASS", ""),
			},
		},
		Admin: admin{
			Active:   StrTo[bool](getEnvOrDefault("ADMIN_ACTIVE", "false")),
			Path:     getEnvOrDefault("ADMIN_PATH", "/admin"),
			Key:      getEnvOrDefault("ADMIN_KEY", ""),
			Password: getEnvOrDefault("ADMIN_PASSWORD", ""),
		},
		Logging: logging{
			Level:       getEnvOrDefault("LOG_LEVEL", "INFO"),
			FilePath:    getEnvOrDefault("LOG_FILE_PATH", ""),
			Stacktrace:  StrTo[bool](getEnvOrDefault("LOG_STACKTRACE", "false")),
			Environment: getEnvOrDefault("LOG_ENVIRONMENT", "development"),
		},
	}

	cfg.Session.Store.Options = cfg.Session.Options
	cfg.Session.Fstore.MaxLength(0)

	if !cfg.Pow.UseCookie && !cfg.Pow.UseSession && !cfg.Pow.UseHeader && cfg.Pow.Active {
		return fmt.Errorf("to use PoW protection you need to setup USE_COOKIE,USE_SESSION or USE_HEADER (both can be true)")
	}

	return nil
}

func loadWafRules(fileName string) string {
	rules := "[]"
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!][Config] Loading waf rule file: %s\n", err.Error())
		return rules
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!][Config] Reading waf rule file: %s\n", err.Error())
		return rules
	}
	return string(b)
}
