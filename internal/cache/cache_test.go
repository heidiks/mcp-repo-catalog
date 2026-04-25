package cache

import (
	"sync"
	"testing"
	"time"
)

func TestSetAndGet(t *testing.T) {
	c := New()

	c.Set("key1", "value1", 5*time.Minute)

	got, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if got != "value1" {
		t.Fatalf("expected value1, got %v", got)
	}
}

func TestGetMissing(t *testing.T) {
	c := New()

	_, ok := c.Get("nonexistent")
	if ok {
		t.Fatal("expected nonexistent key to return false")
	}
}

func TestExpiration(t *testing.T) {
	c := New()

	c.Set("ephemeral", "data", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("ephemeral")
	if ok {
		t.Fatal("expected expired key to return false")
	}
}

func TestLazyEviction(t *testing.T) {
	c := New()

	c.Set("expire-me", "data", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	// Get triggers lazy eviction
	c.Get("expire-me")

	c.mu.RLock()
	_, exists := c.entries["expire-me"]
	c.mu.RUnlock()

	if exists {
		t.Fatal("expected expired entry to be evicted from map")
	}
}

func TestOverwrite(t *testing.T) {
	c := New()

	c.Set("key", "v1", 5*time.Minute)
	c.Set("key", "v2", 5*time.Minute)

	got, ok := c.Get("key")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if got != "v2" {
		t.Fatalf("expected v2, got %v", got)
	}
}

func TestConcurrency(t *testing.T) {
	c := New()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		key := "key"

		go func() {
			defer wg.Done()
			c.Set(key, "value", 5*time.Minute)
		}()

		go func() {
			defer wg.Done()
			c.Get(key)
		}()
	}

	wg.Wait()
}
