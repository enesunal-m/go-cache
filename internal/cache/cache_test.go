package cache

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMultiTierCache(t *testing.T) {
	policy := &LRUPolicy{}
	cache, err := NewMultiTierCache(100, 1000, "localhost:6379", policy)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	ctx := context.Background()

	t.Run("Set and Get", func(t *testing.T) {
		err := cache.Set(ctx, "key1", []byte("value1"))
		if err != nil {
			t.Errorf("Failed to set key1: %v", err)
		}

		value, err := cache.Get(ctx, "key1")
		if err != nil {
			t.Errorf("Failed to get key1: %v", err)
		}
		if string(value) != "value1" {
			t.Errorf("Unexpected value for key1. Got %s, want value1", string(value))
		}
	})

	t.Run("Cache Miss", func(t *testing.T) {
		_, err := cache.Get(ctx, "nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent key, got nil")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := cache.Delete(ctx, "key1")
		if err != nil {
			t.Errorf("Failed to delete key1: %v", err)
		}

		_, err = cache.Get(ctx, "key1")
		if err == nil {
			t.Error("Expected error after deleting key1, got nil")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		err := cache.Set(ctx, "key2", []byte("value2"))
		if err != nil {
			t.Errorf("Failed to set key2: %v", err)
		}

		err = cache.Clear(ctx)
		if err != nil {
			t.Errorf("Failed to clear cache: %v", err)
		}

		_, err = cache.Get(ctx, "key2")
		if err == nil {
			t.Error("Expected error after clearing cache, got nil")
		}
	})

	t.Run("Stats", func(t *testing.T) {
		cache.ResetStats()

		cache.Set(ctx, "key3", []byte("value3"))
		cache.Get(ctx, "key3")
		cache.Get(ctx, "nonexistent")

		hits, misses := cache.GetStats()
		if hits != 1 || misses != 1 {
			t.Errorf("Unexpected stats. Got hits=%d, misses=%d, want hits=1, misses=1", hits, misses)
		}
	})

	t.Run("Eviction", func(t *testing.T) {
		smallCache, _ := NewMultiTierCache(20, 40, "localhost:6379", &LRUPolicy{})

		ctx := context.Background()

		smallCache.Set(ctx, "key1", []byte("value1")) // 6 bytes
		time.Sleep(10 * time.Millisecond)
		smallCache.Set(ctx, "key2", []byte("value2")) // 6 bytes
		time.Sleep(10 * time.Millisecond)
		smallCache.Set(ctx, "key3", []byte("value3")) // 6 bytes
		// Total: 18 bytes

		t.Logf("Memory store usage before setting key4: %d", smallCache.memoryStore.GetUsage())
		t.Logf("Memory store keys before setting key4: %v", smallCache.memoryStore.Keys(ctx))

		// This should trigger an eviction
		t.Logf("Setting key4")
		err := smallCache.Set(ctx, "key4", []byte("longvalue4")) // 10 bytes

		if err != nil {
			t.Errorf("Failed to set key4: %v", err)
		}

		t.Logf("Memory store usage after setting key4: %d", smallCache.memoryStore.GetUsage())
		t.Logf("Memory store keys after setting key4: %v", smallCache.memoryStore.Keys(ctx))

		// key1 should have been evicted from memory but should exist in disk
		_, err = smallCache.memoryStore.Get(ctx, "key1")
		if err == nil {
			t.Error("Expected key1 to be evicted from memory, but it still exists")
		} else {
			t.Logf("key1 was correctly evicted from memory")
		}

		value, err := smallCache.diskStore.Get(ctx, "key1")
		if err != nil || string(value.Value) != "value1" {
			t.Errorf("Expected key1 to exist in disk store with value 'value1', got error: %v, value: %s", err, string(value.Value))
		} else {
			t.Logf("key1 exists in disk store with correct value")
		}

		// key4 should exist in memory
		value, err = smallCache.memoryStore.Get(ctx, "key4")
		if err != nil || string(value.Value) != "longvalue4" {
			t.Errorf("Expected key4 to exist in memory with value 'longvalue4', got error: %v, value: %s", err, string(value.Value))
		} else {
			t.Logf("key4 exists in memory with correct value")
		}

		// Log the keys in the memory and disk stores
		t.Logf("Final keys in memory store: %v", smallCache.memoryStore.Keys(ctx))
		t.Logf("Final keys in disk store: %v", smallCache.diskStore.Keys(ctx))
	})

	// Test promotion
	t.Run("Promotion", func(t *testing.T) {
		promotionCache, _ := NewMultiTierCache(20, 40, "localhost:6379", policy)

		// Set a value in the disk store
		promotionCache.diskStore.Set(ctx, &CacheEntry{Key: "promote", Value: []byte("promotevalue"), Size: 12})

		// Access the value, which should promote it to memory
		value, err := promotionCache.Get(ctx, "promote")
		if err != nil || string(value) != "promotevalue" {
			t.Errorf("Failed to get or promote 'promote' key. Error: %v, Value: %s", err, string(value))
		}

		// Check if the value is now in memory
		_, err = promotionCache.memoryStore.Get(ctx, "promote")
		if err != nil {
			t.Errorf("Expected 'promote' key to be in memory store after promotion, but got error: %v", err)
		}
	})

	t.Run("RemoteStoreMetrics", func(t *testing.T) {
		remoteStore, err := NewRemoteStore("localhost:6379")
		if err != nil {
			t.Fatalf("Failed to create remote store: %v", err)
		}

		ctx := context.Background()

		// Set some data
		remoteStore.Set(ctx, &CacheEntry{Key: "key1", Value: []byte("value1")})
		remoteStore.Set(ctx, &CacheEntry{Key: "key2", Value: []byte("value2")})

		metrics, err := remoteStore.GetMetrics(ctx)
		if err != nil {
			t.Errorf("Failed to get metrics: %v", err)
		}

		if metrics.Capacity <= 0 {
			t.Errorf("Expected positive capacity, got %d", metrics.Capacity)
		}
		if metrics.Usage <= 0 {
			t.Errorf("Expected positive usage, got %d", metrics.Usage)
		}
		if metrics.UsagePercent < 0 || metrics.UsagePercent > 100 {
			t.Errorf("Expected usage percent between 0 and 100, got %f", metrics.UsagePercent)
		}

		capacity := remoteStore.GetCapacity()
		if capacity <= 0 {
			t.Errorf("Expected positive capacity, got %d", capacity)
		}

		usage := remoteStore.GetUsage()
		if usage <= 0 {
			t.Errorf("Expected positive usage, got %d", usage)
		}
	})
}

func TestSimulatedRemoteStore(t *testing.T) {
	os.Setenv("SIMULATE_REMOTE_STORE", "true")
	defer os.Unsetenv("SIMULATE_REMOTE_STORE")

	remoteStore, err := NewRemoteStore("localhost:6379")
	if err != nil {
		t.Fatalf("Failed to create simulated remote store: %v", err)
	}

	ctx := context.Background()

	t.Run("SimulatedSetAndGet", func(t *testing.T) {
		err := remoteStore.Set(ctx, &CacheEntry{Key: "key1", Value: []byte("value1")})
		if err != nil {
			t.Errorf("Failed to set key1: %v", err)
		}

		value, err := remoteStore.Get(ctx, "key1")
		if err != nil {
			t.Errorf("Failed to get key1: %v", err)
		}
		if string(value.Value) != "value1" {
			t.Errorf("Unexpected value for key1. Got %s, want value1", string(value.Value))
		}
	})

	t.Run("SimulatedGetMetrics", func(t *testing.T) {
		metrics, err := remoteStore.GetMetrics(ctx)
		if err != nil {
			t.Errorf("Failed to get metrics: %v", err)
		}

		if metrics.Capacity != 100*1024*1024 { // 100MB
			t.Errorf("Expected capacity of 100MB, got %d bytes", metrics.Capacity)
		}
		if metrics.Usage <= 0 {
			t.Errorf("Expected positive usage, got %d", metrics.Usage)
		}
		if metrics.UsagePercent < 0 || metrics.UsagePercent > 100 {
			t.Errorf("Expected usage percent between 0 and 100, got %f", metrics.UsagePercent)
		}

		capacity := remoteStore.GetCapacity()
		if capacity != 100*1024*1024 { // 100MB
			t.Errorf("Expected capacity of 100MB, got %d bytes", capacity)
		}

		usage := remoteStore.GetUsage()
		if usage <= 0 {
			t.Errorf("Expected positive usage, got %d", usage)
		}
	})
}

func TestNonSimulatedRemoteStore(t *testing.T) {
	// Skip this test if Redis is not available
	if os.Getenv("REDIS_AVAILABLE") != "true" {
		t.Skip("Skipping non-simulated tests as Redis is not available")
	}

	remoteStore, err := NewRemoteStore("localhost:6379")
	if err != nil {
		t.Fatalf("Failed to create non-simulated remote store: %v", err)
	}

	ctx := context.Background()

	t.Run("NonSimulatedSetAndGet", func(t *testing.T) {
		err := remoteStore.Set(ctx, &CacheEntry{Key: "key1", Value: []byte("value1")})
		if err != nil {
			t.Errorf("Failed to set key1: %v", err)
		}

		value, err := remoteStore.Get(ctx, "key1")
		if err != nil {
			t.Errorf("Failed to get key1: %v", err)
		}
		if string(value.Value) != "value1" {
			t.Errorf("Unexpected value for key1. Got %s, want value1", string(value.Value))
		}
	})

	t.Run("NonSimulatedGetMetrics", func(t *testing.T) {
		metrics, err := remoteStore.GetMetrics(ctx)
		if err != nil {
			t.Errorf("Failed to get metrics: %v", err)
		}

		if metrics.Capacity <= 0 {
			t.Errorf("Expected positive capacity, got %d", metrics.Capacity)
		}
		if metrics.Usage <= 0 {
			t.Errorf("Expected positive usage, got %d", metrics.Usage)
		}
		if metrics.UsagePercent < 0 || metrics.UsagePercent > 100 {
			t.Errorf("Expected usage percent between 0 and 100, got %f", metrics.UsagePercent)
		}

		capacity := remoteStore.GetCapacity()
		if capacity <= 0 {
			t.Errorf("Expected positive capacity, got %d", capacity)
		}

		usage := remoteStore.GetUsage()
		if usage <= 0 {
			t.Errorf("Expected positive usage, got %d", usage)
		}
	})
}
