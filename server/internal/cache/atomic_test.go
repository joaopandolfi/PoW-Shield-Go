package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// checkAtomic hammers IncrIfExists and GetDel from many goroutines: no lost increment, a single winner on GetDel
func checkAtomic(t *testing.T, c Cache) {
	const n = 100
	c.Flush() // memory cache is a singleton, start clean on repeated runs
	if _, existed, err := c.IncrIfExists("missing"); err != nil || existed {
		t.Fatalf("missing key: existed=%v err=%v", existed, err)
	}
	if v, _ := c.Get("missing"); v != nil {
		t.Fatal("IncrIfExists created a missing key")
	}

	c.Put("counter", 0, time.Minute)
	seenIncr := map[int]bool{}
	c.Put("once", 1, time.Minute)
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen, winners := map[int]bool{}, 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, existed, err := c.IncrIfExists("counter")
			got, _ := c.GetDel("once")
			fresh, ierr := c.Incr("fresh", 10, 2, time.Minute)
			mu.Lock()
			defer mu.Unlock()
			if ierr != nil || seenIncr[fresh] || fresh%2 != 0 {
				t.Errorf("Incr: v=%d err=%v duplicate=%v", fresh, ierr, seenIncr[fresh])
			}
			seenIncr[fresh] = true
			if err != nil || !existed || seen[v] {
				t.Errorf("incr: v=%d existed=%v err=%v duplicate=%v", v, existed, err, seen[v])
			}
			seen[v] = true
			if got != nil {
				winners++
			}
		}()
	}
	wg.Wait()
	if len(seen) != n || !seen[n] {
		t.Fatalf("expected values 1..%d, got %d distinct", n, len(seen))
	}
	if winners != 1 {
		t.Fatalf("GetDel winners %d, want 1", winners)
	}
	if !seenIncr[12] || !seenIncr[10+2*n] {
		t.Fatalf("Incr from initial 10 by 2 must yield 12..%d", 10+2*n)
	}
}

func TestAtomicMemory(t *testing.T) {
	checkAtomic(t, initializeMemory(context.Background(), time.Minute))
}

func TestAtomicRedis(t *testing.T) {
	mr := miniredis.RunT(t)
	rcache = nil
	c := initializeRedis(context.Background(), mr.Addr(), "", 0)
	defer func() { rcache = nil }()
	checkAtomic(t, c)
	if ttl := mr.TTL("counter"); ttl <= 0 {
		t.Fatalf("IncrIfExists dropped the TTL: %s", ttl)
	}
	if ttl := mr.TTL("fresh"); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("Incr must set the TTL on creation, got %s", ttl)
	}
}
