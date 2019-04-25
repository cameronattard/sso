package groups

import (
	"time"

	"github.com/datadog/datadog-go/statsd"
	"golang.org/x/sync/syncmap"
)

// NewLocalCache returns a LocalCache instance
func NewLocalCache(
	ttl time.Duration,
	statsdClient *statsd.Client,
	tags []string,
) *LocalCache {
	return &LocalCache{
		ttl:            ttl,
		localCacheData: &syncmap.Map{},
		metrics:        statsdClient,
		tags:           tags,
	}
}

type LocalCache struct {
	// Cache configuration
	ttl     time.Duration
	metrics *statsd.Client
	tags    []string

	// Cache data
	localCacheData *syncmap.Map
	Entry
}

// Entry and UserGroupData defines a set of key:value
// pairs to be placed into the cache
type Entry struct {
	Key string
	UserGroupData
}

type UserGroupData struct {
	AllowedGroups []string
	MatchedGroups []string
}

// get will attempt to retrieve an entry from the cache at the given key
func (lc *LocalCache) get(key string) (UserGroupData, bool) {
	var emptycache UserGroupData
	data, found := lc.localCacheData.Load(key)
	if data != nil {
		return data.(UserGroupData), found
	}

	return emptycache, false
}

// set will attempt to set an entry in the cache to a given key
// for the prescribed TTL
func (lc *LocalCache) set(key string, data UserGroupData) error {
	lc.localCacheData.Store(key, data)

	// Spawn the TTL cleanup goroutine if a TTL is set
	if lc.ttl > 0 {
		go func(key string) {
			<-time.After(lc.ttl)
			lc.Purge([]string{key})
		}(key)
	}
	return nil
}

// Get retrieves a key from a local cache. If found, it will create and return an
// 'Entry' using the returned values. If not found, it will return an empty 'Entry'
func (lc *LocalCache) Get(key string) (Entry, error) {
	cached, found := lc.get(key)
	entry := Entry{}
	if found {
		lc.metrics.Incr("localcache.hit", lc.tags, 1.0)
		entry = Entry{
			Key: key,
			UserGroupData: UserGroupData{
				AllowedGroups: cached.AllowedGroups,
				MatchedGroups: cached.MatchedGroups,
			},
		}
	}
	lc.metrics.Incr("localcache.miss", lc.tags, 1.0)

	return entry, nil
}

// Set will set an entry within the current cache
func (lc *LocalCache) Set(entry Entry) (Entry, error) {
	if err := lc.set(entry.Key, entry.UserGroupData); err != nil {
		var emptycache Entry
		lc.metrics.Incr("localcache.set.error", lc.tags, 1.0)
		return emptycache, err
	}
	lc.metrics.Incr("localcache.set.success", lc.tags, 1.0)
	return entry, nil
}

// Purge will remove a set of keys from the local cache map
func (lc *LocalCache) Purge(keys []string) error {
	for _, key := range keys {
		lc.localCacheData.Delete(key)
		//metrics here? https://godoc.org/golang.org/x/sync/syncmap#Map.Delete
	}
	return nil
}
