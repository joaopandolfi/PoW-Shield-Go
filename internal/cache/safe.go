package cache

import (
	"fmt"
	"time"
)

var defaultCacheStoreTime time.Duration = time.Second * 50

type SafeCache[T any] interface {
	Get(key string) *T
	Put(key string, data T) error
	PutDuration(key string, data T, duration time.Duration) error
	Size() int
	Flush() error
	GracefullShutdown()
}

type tCache[T any] struct {
	cache Cache
}

func New[T any]() (tcache SafeCache[T]) {
	t := &tCache[T]{}

	defer lateInitCache(t)
	tcache = t
	t.inject(Get())

	return tcache
}

func (m *tCache[T]) inject(c Cache) {
	m.cache = c
}

func (m *tCache[T]) Get(key string) *T {
	val, _ := m.cache.Get(key)
	if val == nil {
		return nil
	}

	valT, ok := val.(T)
	if !ok {
		return nil
	}

	return &valT
}

func (m *tCache[T]) Put(key string, data T) error {
	return m.PutDuration(key, data, defaultCacheStoreTime)
}

func (m *tCache[T]) PutDuration(key string, data T, duration time.Duration) error {
	err := m.cache.Put(key, data, duration)
	if err != nil {
		return fmt.Errorf("putting data in cache: %w", err)
	}
	return nil
}

func (m *tCache[T]) Size() int {
	if m.cache != nil {
		return m.cache.Size()
	}

	return 0
}

func (m *tCache[T]) Flush() error {
	if m.cache != nil {
		return m.cache.Flush()
	}

	return nil
}

func (m *tCache[T]) GracefullShutdown() {
	if m.cache != nil {
		m.cache.GracefullShutdown()
	}
}
