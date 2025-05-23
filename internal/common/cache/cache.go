package cache

import (
	"time"
)

// A Cache is a generalized interface to a cache.  See cache.LRU for a specific
// implementation (bounded cache with LRU eviction)
type Cache interface {
	// Exist checks if a given key exists in the cache
	Exist(key string) bool

	// Get retrieves an element based on a key, returning nil if the element
	// does not exist
	Get(key string) interface{}

	// Put adds an element to the cache, returning the previous element
	Put(key string, value interface{}) interface{}

	// PutIfNotExist puts a value associated with a given key if it does not exist
	PutIfNotExist(key string, value interface{}) (interface{}, error)

	// Delete deletes an element in the cache
	Delete(key string)

	// Release decrements the ref count of a pinned element. If the ref count
	// drops to 0, the element can be evicted from the cache.
	Release(key string)

	// Size returns the number of entries currently stored in the Cache
	Size() int

	// Clear clears the cache.
	Clear()
}

// Options control the behavior of the cache
type Options struct {
	// TTL controls the time-to-live for a given cache entry.  Cache entries that
	// are older than the TTL will not be returned
	TTL time.Duration

	// InitialCapacity controls the initial capacity of the cache
	InitialCapacity int

	// Pin prevents in-use objects from getting evicted
	Pin bool

	// RemovedFunc is an optional function called when an element
	// is scheduled for deletion
	RemovedFunc RemovedFunc
}

// RemovedFunc is a type for notifying applications when an item is
// scheduled for removal from the Cache. If f is a function with the
// appropriate signature and i is the interface{} scheduled for
// deletion, Cache calls go f(i)
type RemovedFunc func(interface{})
