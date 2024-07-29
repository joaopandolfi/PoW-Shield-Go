//go:build integration

package cache

import (
	"pow-shield-go/config"
	"testing"
	"time"
)

func Test_redis(t *testing.T) {
	cfg := config.Config{}
	cfg.Cache.Redis.Use = true
	cfg.Cache.Redis.Server = "locahost"
	cfg.Cache.Redis.Port = "333"
	cfg.Cache.Redis.Password = "local"
	cfg.Cache.Redis.DB = 1

	config.Inject(cfg)
	GetRedis().Put("teste", 1234, time.Minute*2)
	val, err := GetRedis().Get("teste")
	if err != nil {
		t.Errorf("get redis error: %v", err)
		return
	}
	t.Log(val)
}
