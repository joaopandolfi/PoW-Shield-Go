//go:build integration

package cache

import (
	"context"
	"pow-shield-go/config"
	"testing"
	"time"
)

func TestRedisCache(t *testing.T) {
	cfg := config.Config{}
	cfg.Cache.Redis.Use = true
	cfg.Cache.Redis.Server = "localhost:6379"
	cfg.Cache.Redis.DB = 15

	ctx := context.Background()
	c := initializeRedis(ctx, cfg.Cache.Redis.Server, cfg.Cache.Redis.Password, cfg.Cache.Redis.DB)

	err := c.Put("test-key", "test-value", time.Second*5)
	if err != nil {
		t.Fatalf("failed to put: %v", err)
	}

	val, err := c.Get("test-key")
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if val == nil {
		t.Fatalf("expected value, got nil")
	}

	if val.(string) != "test-value" {
		t.Fatalf("expected test-value, got %v", val)
	}

	err = c.Delete("test-key")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	val, _ = c.Get("test-key")
	if val != nil {
		t.Fatalf("expected nil after delete, got %v", val)
	}

	c.GracefulShutdown()
}
