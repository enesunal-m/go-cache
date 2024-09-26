package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

type CacheEntry struct {
	Key        string
	Value      []byte
	Size       int
	LastAccess time.Time
	Frequency  int
}

type Store interface {
	Get(ctx context.Context, key string) (*CacheEntry, error)
	Set(ctx context.Context, entry *CacheEntry) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	GetCapacity() int
	GetUsage() int
	Keys(ctx context.Context) []string
	GetAll(ctx context.Context) []*CacheEntry
}

type EvictionPolicy interface {
	Choose(entries []*CacheEntry) string
}

type MultiTierCache struct {
	mu sync.RWMutex

	memoryStore Store
	diskStore   Store
	remoteStore Store

	policy EvictionPolicy

	statsHits   int64
	statsMisses int64
}

func NewMultiTierCache(memCap, diskCap int, remoteAddr string, policy EvictionPolicy) (*MultiTierCache, error) {
	memStore := NewMemoryStore(memCap)
	diskStore, err := NewDiskStore(diskCap)
	if err != nil {
		return nil, err
	}
	remoteStore, err := NewRemoteStore(remoteAddr)
	if err != nil {
		return nil, err
	}

	return &MultiTierCache{
		memoryStore: memStore,
		diskStore:   diskStore,
		remoteStore: remoteStore,
		policy:      policy,
	}, nil
}

func (c *MultiTierCache) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, err := c.memoryStore.Get(ctx, key)
	if err == nil {
		c.statsHits++
		entry.LastAccess = time.Now()
		entry.Frequency++
		return entry.Value, nil
	}

	entry, err = c.diskStore.Get(ctx, key)
	if err == nil {
		c.statsHits++
		entry.LastAccess = time.Now()
		entry.Frequency++
		c.promoteToMemory(ctx, entry)
		return entry.Value, nil
	}

	entry, err = c.remoteStore.Get(ctx, key)
	if err == nil {
		c.statsHits++
		entry.LastAccess = time.Now()
		entry.Frequency++
		c.promoteToMemory(ctx, entry)
		return entry.Value, nil
	}

	c.statsMisses++
	return nil, errors.New("key not found")
}

func (c *MultiTierCache) Set(ctx context.Context, key string, value []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		Size:       len(value),
		LastAccess: time.Now(),
		Frequency:  1,
	}

	// Try to set in memory first
	err := c.memoryStore.Set(ctx, entry)
	if err == nil {
		return nil
	}

	// If memory is full, try to evict
	if errors.Is(err, ErrInsufficientCapacity) {
		evicted := c.evict(ctx, c.memoryStore, entry.Size)
		if evicted {
			err = c.memoryStore.Set(ctx, entry)
			if err == nil {
				return nil
			}
		}
	}

	// If still can't fit in memory, try disk
	err = c.diskStore.Set(ctx, entry)
	if err == nil {
		return nil
	}

	// If disk is full, try to evict
	if errors.Is(err, ErrInsufficientCapacity) {
		evicted := c.evict(ctx, c.diskStore, entry.Size)
		if evicted {
			err = c.diskStore.Set(ctx, entry)
			if err == nil {
				return nil
			}
		}
	}

	// If still can't fit, use remote store
	return c.remoteStore.Set(ctx, entry)
}

func (c *MultiTierCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.memoryStore.Delete(ctx, key)
	c.diskStore.Delete(ctx, key)
	return c.remoteStore.Delete(ctx, key)
}

func (c *MultiTierCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.memoryStore.Clear(ctx)
	c.diskStore.Clear(ctx)
	return c.remoteStore.Clear(ctx)
}

func (c *MultiTierCache) promoteToMemory(ctx context.Context, entry *CacheEntry) {
	if c.memoryStore.GetUsage()+entry.Size > c.memoryStore.GetCapacity() {
		c.evict(ctx, c.memoryStore, entry.Size)
	}
	c.memoryStore.Set(ctx, entry)
}

func (c *MultiTierCache) evict(ctx context.Context, store Store, requiredSpace int) bool {
	for store.GetCapacity()-store.GetUsage() < requiredSpace {
		entries := store.GetAll(ctx)
		if len(entries) == 0 {
			return false
		}
		keyToEvict := c.policy.Choose(entries)
		if keyToEvict == "" {
			return false
		}
		evictedEntry, _ := store.Get(ctx, keyToEvict)
		store.Delete(ctx, keyToEvict)
		if evictedEntry != nil {
			c.promoteEvictedEntry(ctx, evictedEntry)
		}
	}
	return true
}

func (c *MultiTierCache) promoteEvictedEntry(ctx context.Context, entry *CacheEntry) {
	if store, ok := c.getNextTier(entry); ok {
		store.Set(ctx, entry)
	}
}

func (c *MultiTierCache) getNextTier(entry *CacheEntry) (Store, bool) {
	switch {
	case entry.Size <= c.memoryStore.GetCapacity():
		return c.diskStore, true
	case entry.Size <= c.diskStore.GetCapacity():
		return c.remoteStore, true
	default:
		return nil, false
	}
}

func (c *MultiTierCache) getEntries(ctx context.Context, store Store) []*CacheEntry {
	var entries []*CacheEntry
	for _, key := range c.Keys(ctx) {
		if entry, err := store.Get(ctx, key); err == nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (c *MultiTierCache) Keys(ctx context.Context) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var keys []string
	for _, store := range []Store{c.memoryStore, c.diskStore, c.remoteStore} {
		if keysGetter, ok := store.(interface {
			Keys(context.Context) []string
		}); ok {
			keys = append(keys, keysGetter.Keys(ctx)...)
		}
	}
	return keys
}

func (c *MultiTierCache) GetStats() (hits, misses int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.statsHits, c.statsMisses
}

func (c *MultiTierCache) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.statsHits = 0
	c.statsMisses = 0
}

func (c *MultiTierCache) MemoryStore() Store {
	return c.memoryStore
}

func (c *MultiTierCache) DiskStore() Store {
	return c.diskStore
}

func (c *MultiTierCache) RemoteStore() Store {
	return c.remoteStore
}
