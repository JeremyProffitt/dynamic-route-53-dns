package route53

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

var (
	client *route53.Client
	once   sync.Once
)

// Cache for zone data
type zoneCache struct {
	zones     []Zone
	fetchedAt time.Time
	mu        sync.RWMutex
}

var cache = &zoneCache{}

const cacheTTL = 5 * time.Minute

// Init initializes the Route 53 client
func Init(ctx context.Context) error {
	var initErr error
	once.Do(func() {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			initErr = err
			return
		}
		client = route53.NewFromConfig(cfg)
	})
	return initErr
}

// GetClient returns the Route 53 client
func GetClient() *route53.Client {
	return client
}

// isCacheValid checks if the cache is still valid
func isCacheValid() bool {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.zones != nil && time.Since(cache.fetchedAt) < cacheTTL
}

// getCachedZones returns cached zones if valid
func getCachedZones() []Zone {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	if cache.zones != nil && time.Since(cache.fetchedAt) < cacheTTL {
		return cache.zones
	}
	return nil
}

// setCachedZones updates the cache
func setCachedZones(zones []Zone) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.zones = zones
	cache.fetchedAt = time.Now()
}

// InvalidateCache clears the zone cache
func InvalidateCache() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.zones = nil
}
