package cache

import (
	"context"
	"pow-shield-go/config"
	"time"
)

// ponytail: past this size every Put scans the map dropping expired entries (valid ones are never evicted); use Redis for real traffic
const MAX_BUFF_SIZE = 100_000

var cacheInstance Cache

var InitializedChan chan bool = make(chan bool, 2)

var waitListenners []chan bool

type Cache interface {
	Put(key string, data interface{}, duration time.Duration) error
	// PutIfAbsent stores only when key is missing, atomically; returns false when it already existed
	PutIfAbsent(key string, data interface{}, duration time.Duration) (bool, error)
	// IncrIfExists atomically adds 1 to an existing integer key, keeping its TTL; existed is false (and nothing is created) when missing
	IncrIfExists(key string) (value int, existed bool, err error)
	// Incr atomically creates key with initial (and ttl) when missing, then adds delta; ttl is only set on creation
	Incr(key string, initial, delta int, ttl time.Duration) (int, error)
	// GetDel atomically reads and removes key, so only one caller gets the value
	GetDel(key string) (interface{}, error)
	Get(key string) (interface{}, error)
	Delete(key string) error
	Size() int
	Flush() error
	GracefulShutdown()
}

func Initialize(ctx context.Context, tick time.Duration) Cache {
	if config.Get().Cache.Redis.Use {
		cacheInstance = GetRedis(ctx)
	} else {
		cacheInstance = initializeMemory(ctx, tick)
	}
	initialized()
	return cacheInstance
}

func AddInitializedListenner(l chan bool) {
	if waitListenners == nil {
		waitListenners = []chan bool{}
	}
	waitListenners = append(waitListenners, l)
}

func initialized() {
	InitializedChan <- true
	for _, c := range waitListenners {
		c <- true
	}
	waitListenners = nil
}

// Ping checks the backend is reachable, memory is always up
func Ping(ctx context.Context) error {
	if r, ok := cacheInstance.(*redisCache); ok {
		return r.client.Ping(ctx).Err()
	}
	return nil
}

func Get() Cache {
	if cacheInstance == nil {
		panic("cache not initialized")
	}
	return cacheInstance
}
