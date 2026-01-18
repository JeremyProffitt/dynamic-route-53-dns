package route53

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

const (
	// CacheTTL is the duration for which cached data remains valid
	CacheTTL = 5 * time.Minute
)

// cacheEntry represents a cached item with its expiration time
type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

// isValid checks if the cache entry is still valid
func (c *cacheEntry) isValid() bool {
	return time.Now().Before(c.expiresAt)
}

// Route53Client wraps the AWS Route53 client with caching capabilities
type Route53Client struct {
	client *route53.Client
	cache  sync.Map
}

// NewClient creates a new Route53Client using the default AWS configuration
func NewClient(ctx context.Context) (*Route53Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	client := route53.NewFromConfig(cfg)

	return &Route53Client{
		client: client,
	}, nil
}

// getFromCache retrieves an item from the cache if it exists and is valid
func (r *Route53Client) getFromCache(key string) (interface{}, bool) {
	value, ok := r.cache.Load(key)
	if !ok {
		return nil, false
	}

	entry, ok := value.(*cacheEntry)
	if !ok {
		return nil, false
	}

	if !entry.isValid() {
		r.cache.Delete(key)
		return nil, false
	}

	return entry.data, true
}

// setInCache stores an item in the cache with the default TTL
func (r *Route53Client) setInCache(key string, data interface{}) {
	entry := &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(CacheTTL),
	}
	r.cache.Store(key, entry)
}

// invalidateCache removes an item from the cache
func (r *Route53Client) invalidateCache(key string) {
	r.cache.Delete(key)
}

// invalidateAllCache clears all cached data
func (r *Route53Client) invalidateAllCache() {
	r.cache.Range(func(key, value interface{}) bool {
		r.cache.Delete(key)
		return true
	})
}
