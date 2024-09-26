package cache

import (
	"context"
	"encoding/gob"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

type DiskStore struct {
	mu       sync.RWMutex
	dir      string
	capacity int
	usage    int
}

func NewDiskStore(capacity int) (*DiskStore, error) {
	dir, err := os.MkdirTemp("", "diskcache")
	if err != nil {
		return nil, err
	}

	return &DiskStore{
		dir:      dir,
		capacity: capacity,
	}, nil
}

func (s *DiskStore) Get(_ context.Context, key string) (*CacheEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dir, key)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entry CacheEntry
	if err := gob.NewDecoder(file).Decode(&entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

func (s *DiskStore) Set(_ context.Context, entry *CacheEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.usage+entry.Size > s.capacity {
		return errors.New("insufficient capacity")
	}

	path := filepath.Join(s.dir, entry.Key)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := gob.NewEncoder(file).Encode(entry); err != nil {
		return err
	}

	s.usage += entry.Size
	return nil
}

func (s *DiskStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, key)
	info, err := os.Stat(path)
	if err == nil {
		s.usage -= int(info.Size())
		return os.Remove(path)
	}
	return nil
}

func (s *DiskStore) Clear(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.RemoveAll(s.dir); err != nil {
		return err
	}
	s.usage = 0
	return os.MkdirAll(s.dir, 0755)
}

func (s *DiskStore) GetCapacity() int {
	return s.capacity
}

func (s *DiskStore) GetUsage() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.usage
}

func (s *DiskStore) Keys(_ context.Context) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	files, _ := ioutil.ReadDir(s.dir)
	for _, file := range files {
		keys = append(keys, file.Name())
	}
	return keys
}

func (s *DiskStore) GetAll(_ context.Context) []*CacheEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []*CacheEntry
	files, _ := ioutil.ReadDir(s.dir)
	for _, file := range files {
		path := filepath.Join(s.dir, file.Name())
		if entry, err := s.readEntry(path); err == nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (s *DiskStore) readEntry(path string) (*CacheEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entry CacheEntry
	if err := gob.NewDecoder(file).Decode(&entry); err != nil {
		return nil, err
	}

	return &entry, nil
}
