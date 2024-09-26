# Multi-Tier Cache

## Overview

Multi-Tier Cache is a Go implementation of a caching system that utilizes multiple storage tiers: memory, disk, and remote (Redis). This project provides a flexible and efficient caching solution that can be easily integrated into various applications.

## Features

- Multiple storage tiers: memory, disk, and remote (Redis)
- Configurable eviction policies
- Thread-safe operations
- Simulation mode for testing without a Redis connection
- Extensible design for adding new storage types or eviction policies

## Installation

To use this cache in your Go project, you can clone this repository or import it as a module:

```bash
go get github.com/yourusername/multi-tier-cache
```

Make sure you have Go installed (version 1.16 or later is recommended).

## Usage

Here's a basic example of how to use the Multi-Tier Cache:

```go
package main

import (
    "context"
    "fmt"
    "log"

    cache "github.com/yourusername/multi-tier-cache"
)

func main() {
    ctx := context.Background()

    // Create a new multi-tier cache
    c, err := cache.NewMultiTierCache(100, 1000, "localhost:6379", &cache.LRUPolicy{})
    if err != nil {
        log.Fatalf("Error creating cache: %v", err)
    }

    // Set a value
    err = c.Set(ctx, "key1", []byte("value1"))
    if err != nil {
        log.Printf("Error setting value: %v", err)
    }

    // Get a value
    val, err := c.Get(ctx, "key1")
    if err != nil {
        log.Printf("Error getting value: %v", err)
    } else {
        fmt.Printf("Value for key1: %s\n", string(val))
    }

    // Delete a value
    err = c.Delete(ctx, "key1")
    if err != nil {
        log.Printf("Error deleting value: %v", err)
    }

    // Print cache stats
    hits, misses := c.GetStats()
    fmt.Printf("Cache stats - Hits: %d, Misses: %d\n", hits, misses)
}
```

## Components

### MultiTierCache

The main struct that orchestrates the caching system. It manages the different storage tiers and implements the caching logic.

### MemoryStore

An in-memory storage implementation using a Go map.

### DiskStore

A disk-based storage implementation that persists cache entries to the file system.

### RemoteStore

A Redis-based storage implementation that can also simulate Redis operations for testing purposes.

### EvictionPolicy

An interface for implementing different cache eviction policies. The project includes an LRU (Least Recently Used) policy implementation.

## Configuration

The `NewMultiTierCache` function accepts the following parameters:

- `memCap`: Capacity of the memory store in bytes
- `diskCap`: Capacity of the disk store in bytes
- `remoteAddr`: Address of the Redis server (e.g., "localhost:6379")
- `policy`: An implementation of the `EvictionPolicy` interface

## Simulating Remote Store

To simulate the remote store without an actual Redis connection, set the `SIMULATE_REMOTE_STORE` environment variable to "true":

```bash
export SIMULATE_REMOTE_STORE=true
```

This is useful for testing and development when a Redis instance is not available.

## Thread Safety

All operations in the Multi-Tier Cache are thread-safe. The cache uses mutexes to ensure safe concurrent access to the stored data.

## Extending the Cache

### Adding a New Storage Type

To add a new storage type:

1. Implement the `Store` interface for your new storage type.
2. Modify the `MultiTierCache` struct and `NewMultiTierCache` function to include your new store.
3. Update the caching logic in `MultiTierCache` methods to utilize the new store.

### Implementing a New Eviction Policy

To add a new eviction policy:

1. Implement the `EvictionPolicy` interface with your new policy logic.
2. Pass an instance of your new policy to `NewMultiTierCache` when creating a cache instance.

## Contributing

Contributions to the Multi-Tier Cache project are welcome! Please feel free to submit issues, fork the repository and send pull requests!

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
