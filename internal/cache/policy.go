package cache

import (
	"time"
)

type LRUPolicy struct{}

func (p *LRUPolicy) Choose(entries []*CacheEntry) string {
	if len(entries) == 0 {
		return ""
	}

	oldestAccess := time.Now()
	oldestKey := ""

	for _, entry := range entries {
		if entry.LastAccess.Before(oldestAccess) {
			oldestAccess = entry.LastAccess
			oldestKey = entry.Key
		}
	}

	return oldestKey
}
