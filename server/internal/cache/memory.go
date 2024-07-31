package cache

import (
	"context"
	"log"
	"sync"
	"time"

	"fmt"
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
		log.Println("[CACHE] using local cache", "Memory")
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
	if len(c.buff) > MAX_BUFF_SIZE {
		return fmt.Errorf("buffer overflow")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.buff[key] = &stored{
		value:   data,
		validAt: time.Now().Add(duration),
	}
	return nil
}

func (c *memCache) Get(key string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.buff)
}

// ==== GARBAGE COLLECTOR -> PUT THIS IN A SEPARATED STRUCTURE

func (c *memCache) startGarbageCollector(tick time.Duration) {
	ticker := time.NewTicker(tick)
	c.garbageStop = make(chan bool)

	go func() {
		log.Println("[LOCAL_CACHE][GARBAGE COLLECTOR] - START - Garbage tick seconds:", tick.Seconds())
		c.garbageInitialized <- true
		for {
			select {
			case <-c.garbageStop:
				ticker.Stop()
				log.Println("[LOCAL_CACHE][GARBAGE COLLECTOR] - STOP")
				return
			case <-ticker.C:
				c.GarbageCollector()
			case <-c.ctx.Done():
				log.Println("[LOCAL_CACHE][Context done] Calling gracefullShutdown")
				c.GracefullShutdown()
			}
		}
	}()
}

func (c *memCache) GarbageCollector() {
	var toDelete []string
	for k, val := range c.buff {
		if val.validAt.Before(time.Now()) {
			toDelete = append(toDelete, k)
		}
	}

	for _, d := range toDelete {
		c.Delete(d)
	}
}

func (c *memCache) GracefullShutdown() {
	if c.garbageStop != nil {
		c.garbageStop <- true
	}
}
