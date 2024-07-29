package cache

import (
	"context"
	"fmt"
	"log"
	"pow-shield-go/config"
	"time"

	"github.com/go-redis/redis/v8"
)

var rcache *redisCache

type redisCache struct {
	client *redis.Client
	ctx    context.Context
}

func GetRedis(ctx context.Context) Cache {
	cfg := config.Get().Cache.Redis
	return initializeRedis(ctx, cfg.Server, cfg.Password, cfg.DB)
}

func initializeRedis(ctx context.Context, server, password string, db int) Cache {
	if rcache == nil {
		log.Println("[CACHE] using redis cache")

		rcache = &redisCache{
			ctx: ctx,
			client: redis.NewClient(&redis.Options{
				Addr:     server,
				Password: password,
				DB:       db,
			}),
		}
	}

	go rcache.ctxHandlerCloser()

	return rcache
}

func (c *redisCache) Put(key string, data interface{}, duration time.Duration) error {
	err := c.client.Set(c.ctx, key, data, duration).Err()
	if err != nil {
		return fmt.Errorf("putting data on key (%s): %w", key, err)
	}
	return nil
}

func (c *redisCache) Get(key string) (interface{}, error) {
	val, err := c.client.Get(c.ctx, "key").Result()
	if err == redis.Nil {
		return nil, nil // Key does not exists
	} else if err != nil {
		return nil, fmt.Errorf("getting data from %s : %w", key, err)
	}
	return val, nil
}

func (c *redisCache) Delete(key string) error {
	c.client.Del(c.ctx, key)
	return nil
}
func (c *redisCache) Size() int {
	return 0
}
func (c *redisCache) Flush() error {
	return nil
}
func (c *redisCache) GracefullShutdown() {
	c.client.Close()
}

func (c *redisCache) ctxHandlerCloser() {
	for {
		<-c.ctx.Done()
		log.Println("[REDIS_CACHE][Context done] Calling gracefullShutdown")
		c.GracefullShutdown()
		return
	}
}
