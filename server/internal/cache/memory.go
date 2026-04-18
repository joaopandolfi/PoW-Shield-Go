package cache

import (
	"context"
	"sync"
	"time"

	"pow-shield-go/internal/logging"
)

var mcache *memCache

type memCache struct {
	buff               map[string]*stored
	garbageStop        chan bool
	mu                 sync.RWMutex
	garbageInitialized chan bool
	ctx                context.Context
	// improve : use sync.Map
}

func GetMemory() Cache {
	return mcache
}

func initializeMemory(ctx context.Context, tick time.Duration) Cache {
	if mcache == nil {
		log := logging.Get()
		if log != nil {
			log.Debug("Using local cache", "backend", "Memory")
		}
		mcache = &memCache{
			buff:               map[string]*stored{},
			garbageInitialized: make(chan bool, 1),
			ctx:                ctx,
		}

		mcache.startGarbageCollector(tick)
		<-mcache.garbageInitialized
		close(mcache.garbageInitialized)
	}
	return mcache
}

func (c *memCache) Put(key string, data interface{}, duration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for len(c.buff) > MAX_BUFF_SIZE {
		oldestKey := ""
		oldestTime := time.Now()
		for k, v := range c.buff {
			if v.validAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.validAt
			}
		}
		if oldestKey != "" {
			delete(c.buff, oldestKey)
		} else {
			break
		}
	}

	c.buff[key] = &stored{
		value:   data,
		validAt: time.Now().Add(duration),
	}
	return nil
}

func (c *memCache) Get(key string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if val, ok := c.buff[key]; ok {
		now := time.Now()
		if val.validAt.After(now) {
			return val.value, nil
		}
	}
	return nil, nil
}

func (c *memCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buff[key] = nil
	delete(c.buff, key)
	return nil
}

func (c *memCache) Flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buff = map[string]*stored{}
	return nil
}

func (c *memCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.buff)
}

// ==== GARBAGE COLLECTOR -> PUT THIS IN A SEPARATED STRUCTURE

func (c *memCache) startGarbageCollector(tick time.Duration) {
	ticker := time.NewTicker(tick)
	c.garbageStop = make(chan bool)

	go func() {
		log := logging.Get()
		if log != nil {
			log.Debug("Garbage collector started", "tick_seconds", tick.Seconds())
		}
		c.garbageInitialized <- true
		for {
			select {
			case <-c.garbageStop:
				ticker.Stop()
				if log != nil {
					log.Debug("Garbage collector stopped")
				}
				return
			case <-ticker.C:
				c.GarbageCollector()
			case <-c.ctx.Done():
				if log != nil {
					log.Warn("Cache context done, calling gracefulShutdown")
				}
				c.GracefulShutdown()
			}
		}
	}()
}

func (c *memCache) GarbageCollector() {
	var toDelete []string
	c.mu.RLock()
	for k, val := range c.buff {
		if val.validAt.Before(time.Now()) {
			toDelete = append(toDelete, k)
		}
	}
	c.mu.RUnlock()

	for _, d := range toDelete {
		c.Delete(d)
	}
}

func (c *memCache) GracefulShutdown() {
	if c.garbageStop != nil {
		c.garbageStop <- true
	}
}
