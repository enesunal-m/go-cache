package main

import (
	"context"
	"fmt"

	"go-cache/internal/cache"
)

func main() {
	ctx := context.Background()

	// Create a new multi-tier cache
	c, err := cache.NewMultiTierCache(100, 1000, "localhost:6379", &cache.LRUPolicy{})
	if err != nil {
		fmt.Printf("Error creating cache: %v\n", err)
		return
	}

	// Set some values
	c.Set(ctx, "key1", []byte("value1"))
	c.Set(ctx, "key2", []byte("value2"))

	// Get a value
	if val, err := c.Get(ctx, "key1"); err == nil {
		fmt.Printf("key1: %s\n", string(val))
	} else {
		fmt.Printf("Error getting key1: %v\n", err)
	}

	// Print cache stats
	hits, misses := c.GetStats()
	fmt.Printf("Cache stats - Hits: %d, Misses: %d\n", hits, misses)

	// Demonstrate batch operations
	batchSet := map[string][]byte{
		"key3": []byte("value3"),
		"key4": []byte("value4"),
	}
	for k, v := range batchSet {
		c.Set(ctx, k, v)
	}

	// Get multiple values
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	for _, k := range keys {
		if val, err := c.Get(ctx, k); err == nil {
			fmt.Printf("%s: %s\n", k, string(val))
		} else {
			fmt.Printf("%s: Not found\n", k)
		}
	}

	// Demonstrate eviction
	smallCache, _ := cache.NewMultiTierCache(20, 40, "localhost:6379", &cache.LRUPolicy{})

	fmt.Println("\nDemonstrating eviction:")
	smallCache.Set(ctx, "key1", []byte("value1"))
	smallCache.Set(ctx, "key2", []byte("value2"))
	smallCache.Set(ctx, "key3", []byte("value3"))

	fmt.Println("Cache state after setting key1, key2, key3:")
	printCacheState(ctx, smallCache)

	fmt.Println("Setting key4 (should trigger eviction):")
	smallCache.Set(ctx, "key4", []byte("longvalue4"))

	fmt.Println("Cache state after setting key4:")
	printCacheState(ctx, smallCache)
}

func printCacheState(ctx context.Context, c *cache.MultiTierCache) {
	fmt.Println("Memory store:")
	for _, key := range c.MemoryStore().Keys(ctx) {
		val, _ := c.MemoryStore().Get(ctx, key)
		fmt.Printf("  %s: %s\n", key, string(val.Value))
	}

	fmt.Println("Disk store:")
	for _, key := range c.DiskStore().Keys(ctx) {
		val, _ := c.DiskStore().Get(ctx, key)
		fmt.Printf("  %s: %s\n", key, string(val.Value))
	}
}
