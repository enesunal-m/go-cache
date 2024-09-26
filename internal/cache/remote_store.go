package cache

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

type RemoteStore struct {
	simulate    bool
	client      *redis.Client
	simulateMap map[string][]byte
	mu          sync.RWMutex
}

type StoreMetrics struct {
	Capacity     int64   // in bytes
	Usage        int64   // in bytes
	UsagePercent float64 // percentage of capacity used
}

func NewRemoteStore(addr string) (*RemoteStore, error) {
	simulate, ok := os.LookupEnv("SIMULATE_REMOTE_STORE")
	if ok && simulate == "true" {
		log.Println("Simulating remote store connection")
		return &RemoteStore{
			simulate:    true,
			simulateMap: make(map[string][]byte),
		}, nil
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		return nil, err
	}
	log.Println("Connected to remote store")
	return &RemoteStore{client: client}, nil
}

func (s *RemoteStore) Get(ctx context.Context, key string) (*CacheEntry, error) {
	if s.simulate {
		s.mu.RLock()
		defer s.mu.RUnlock()
		if val, ok := s.simulateMap[key]; ok {
			log.Println("Simulating GET request to remote store")
			return &CacheEntry{Key: key, Value: val}, nil
		}
		return nil, redis.Nil
	}
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	return &CacheEntry{Key: key, Value: []byte(val)}, nil
}

func (s *RemoteStore) Set(ctx context.Context, entry *CacheEntry) error {
	if s.simulate {
		s.mu.Lock()
		defer s.mu.Unlock()
		log.Println("Simulating SET request to remote store")
		s.simulateMap[entry.Key] = entry.Value
		return nil
	}
	return s.client.Set(ctx, entry.Key, entry.Value, 0).Err()
}

func (s *RemoteStore) Delete(ctx context.Context, key string) error {
	if s.simulate {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.simulateMap, key)
		return nil
	}
	return s.client.Del(ctx, key).Err()
}

func (s *RemoteStore) Clear(ctx context.Context) error {
	if s.simulate {
		s.mu.Lock()
		defer s.mu.Unlock()
		log.Println("Simulating CLEAR request to remote store")
		s.simulateMap = make(map[string][]byte)
		return nil
	}
	return s.client.FlushDB(ctx).Err()
}

func (s *RemoteStore) Keys(ctx context.Context) []string {
	if s.simulate {
		s.mu.RLock()
		defer s.mu.RUnlock()
		log.Println("Simulating KEYS request to remote store")
		keys := make([]string, 0, len(s.simulateMap))
		for k := range s.simulateMap {
			keys = append(keys, k)
		}
		return keys
	}
	keys, err := s.client.Keys(ctx, "*").Result()
	if err != nil {
		return []string{}
	}
	return keys
}

func (s *RemoteStore) GetAll(ctx context.Context) []*CacheEntry {
	if s.simulate {
		s.mu.RLock()
		defer s.mu.RUnlock()
		log.Println("Simulating GETALL request to remote store")
		entries := make([]*CacheEntry, 0, len(s.simulateMap))
		for k, v := range s.simulateMap {
			entries = append(entries, &CacheEntry{Key: k, Value: v})
		}
		return entries
	}
	keys, err := s.client.Keys(ctx, "*").Result()
	if err != nil {
		return []*CacheEntry{}
	}
	entries := make([]*CacheEntry, 0, len(keys))
	for _, key := range keys {
		val, err := s.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		entries = append(entries, &CacheEntry{Key: key, Value: []byte(val)})
	}
	return entries
}

func (s *RemoteStore) GetMetrics(ctx context.Context) (StoreMetrics, error) {
	if s.simulate {
		s.mu.RLock()
		defer s.mu.RUnlock()
		usage := int64(0)
		for _, v := range s.simulateMap {
			usage += int64(len(v))
		}
		capacity := int64(1024 * 1024 * 100) // Simulate 100MB capacity
		return StoreMetrics{
			Capacity:     capacity,
			Usage:        usage,
			UsagePercent: float64(usage) / float64(capacity) * 100,
		}, nil
	}

	// Get the maximum memory limit set for Redis
	maxMemoryConfig, err := s.client.ConfigGet(ctx, "maxmemory").Result()
	if err != nil {
		return StoreMetrics{}, fmt.Errorf("failed to get maxmemory: %w", err)
	}

	maxMemoryStr, ok := maxMemoryConfig["maxmemory"]
	if !ok {
		return StoreMetrics{}, fmt.Errorf("maxmemory not found in Redis configuration")
	}

	capacity, err := strconv.ParseInt(maxMemoryStr, 10, 64)
	if err != nil {
		return StoreMetrics{}, fmt.Errorf("failed to parse maxmemory: %w", err)
	}

	// Get the current memory usage of Redis
	info, err := s.client.Info(ctx, "memory").Result()
	if err != nil {
		return StoreMetrics{}, fmt.Errorf("failed to get memory info: %w", err)
	}

	var usage int64
	for _, line := range strings.Split(info, "\r\n") {
		if strings.HasPrefix(line, "used_memory:") {
			usedMemory := strings.TrimPrefix(line, "used_memory:")
			usage, err = strconv.ParseInt(usedMemory, 10, 64)
			if err != nil {
				return StoreMetrics{}, fmt.Errorf("failed to parse used_memory: %w", err)
			}
			break
		}
	}

	if usage == 0 {
		return StoreMetrics{}, fmt.Errorf("failed to find used_memory in Redis info")
	}

	usagePercent := float64(usage) / float64(capacity) * 100

	return StoreMetrics{
		Capacity:     capacity,
		Usage:        usage,
		UsagePercent: usagePercent,
	}, nil
}

func (s *RemoteStore) GetCapacity() int {
	metrics, err := s.GetMetrics(context.Background())
	if err != nil {
		log.Printf("Error getting capacity: %v", err)
		return -1
	}
	return int(metrics.Capacity)
}

func (s *RemoteStore) GetUsage() int {
	metrics, err := s.GetMetrics(context.Background())
	if err != nil {
		log.Printf("Error getting usage: %v", err)
		return 0
	}
	return int(metrics.Usage)
}
