package cache

import (
	"context"
	"errors"
	"sync"
)

var ErrInsufficientCapacity = errors.New("insufficient capacity")

type MemoryStore struct {
	mu       sync.RWMutex
	items    map[string]*CacheEntry
	capacity int
	usage    int
}

func NewMemoryStore(capacity int) *MemoryStore {
	return &MemoryStore{
		items:    make(map[string]*CacheEntry),
		capacity: capacity,
	}
}

func (s *MemoryStore) Get(_ context.Context, key string) (*CacheEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if entry, ok := s.items[key]; ok {
		return entry, nil
	}
	return nil, errors.New("key not found")
}

func (s *MemoryStore) Set(_ context.Context, entry *CacheEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newUsage := s.usage + entry.Size
	if existing, ok := s.items[entry.Key]; ok {
		newUsage -= existing.Size
	}

	if newUsage > s.capacity {
		return ErrInsufficientCapacity
	}

	s.items[entry.Key] = entry
	s.usage = newUsage
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.items[key]; ok {
		s.usage -= entry.Size
		delete(s.items, key)
	}
	return nil
}

func (s *MemoryStore) Clear(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = make(map[string]*CacheEntry)
	s.usage = 0
	return nil
}

func (s *MemoryStore) GetCapacity() int {
	return s.capacity
}

func (s *MemoryStore) GetUsage() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.usage
}

func (s *MemoryStore) Keys(_ context.Context) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.items))
	for k := range s.items {
		keys = append(keys, k)
	}
	return keys
}

func (s *MemoryStore) GetAll(_ context.Context) []*CacheEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]*CacheEntry, 0, len(s.items))
	for _, v := range s.items {
		entries = append(entries, v)
	}
	return entries
}
