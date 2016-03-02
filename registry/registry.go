package registry

import (
	"encoding/json"
	"sync"
	"time"
	"net"

	steno "github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/yagnats"

	"github.com/nimbus-cloud/gorouter/config"
	"github.com/nimbus-cloud/gorouter/route"
)

type RegistryInterface interface {
	Register(uri route.Uri, endpoint *route.Endpoint)
	Unregister(uri route.Uri, endpoint *route.Endpoint)
	Lookup(uri route.Uri) *route.Pool
	StartPruningCycle()
	StopPruningCycle()
	NumUris() int
	NumEndpoints() int
	MarshalJSON() ([]byte, error)
}

type RouteRegistry struct {
	sync.RWMutex

	logger *steno.Logger

	byUri *Trie

	pruneStaleDropletsInterval time.Duration
	dropletStaleThreshold      time.Duration

	messageBus yagnats.NATSConn

	ticker           *time.Ticker
	timeOfLastUpdate time.Time

	preferredNetwork *net.IPNet
}

func NewRouteRegistry(c *config.Config, mbus yagnats.NATSConn) *RouteRegistry {
	r := &RouteRegistry{}

	r.logger = steno.NewLogger("router.registry")

	r.byUri = NewTrie()

	r.pruneStaleDropletsInterval = c.PruneStaleDropletsInterval
	r.dropletStaleThreshold = c.DropletStaleThreshold

	r.preferredNetwork = c.PreferredNetwork

	r.messageBus = mbus

	return r
}

func (r *RouteRegistry) Register(uri route.Uri, endpoint *route.Endpoint) {
	t := time.Now()
	r.Lock()

	uri = uri.ToLower()

	pool, found := r.byUri.Find(uri)
	if !found {
		pool = route.NewPool(r.dropletStaleThreshold / 4, r.preferredNetwork)
		r.byUri.Insert(uri, pool)
	}

	pool.Put(endpoint)

	r.timeOfLastUpdate = t
	r.Unlock()
}

func (r *RouteRegistry) Unregister(uri route.Uri, endpoint *route.Endpoint) {
	r.Lock()

	uri = uri.ToLower()

	pool, found := r.byUri.Find(uri)
	if found {
		pool.Remove(endpoint)

		if pool.IsEmpty() {
			r.byUri.Delete(uri)
		}
	}

	r.Unlock()
}

func (r *RouteRegistry) Lookup(uri route.Uri) *route.Pool {
	r.RLock()

	uri = uri.ToLower()
	var err error
	pool, found := r.byUri.MatchUri(uri)
	for !found && err == nil {
		uri, err = uri.NextWildcard()
		pool, found = r.byUri.MatchUri(uri)
	}

	r.RUnlock()

	return pool
}

func (r *RouteRegistry) StartPruningCycle() {
	if r.pruneStaleDropletsInterval > 0 {
		r.Lock()
		r.ticker = time.NewTicker(r.pruneStaleDropletsInterval)
		r.Unlock()

		go func() {
			for {
				select {
				case <-r.ticker.C:
					r.logger.Debug("Start to check and prune stale droplets")
					r.pruneStaleDroplets()
				}
			}
		}()
	}
}

func (r *RouteRegistry) StopPruningCycle() {
	r.Lock()
	if r.ticker != nil {
		r.ticker.Stop()
	}
	r.Unlock()
}

func (registry *RouteRegistry) NumUris() int {
	registry.RLock()
	uriCount := registry.byUri.PoolCount()
	registry.RUnlock()

	return uriCount
}

func (r *RouteRegistry) TimeOfLastUpdate() time.Time {
	r.RLock()
	t := r.timeOfLastUpdate
	r.RUnlock()

	return t
}

func (r *RouteRegistry) NumEndpoints() int {
	r.RLock()
	count := r.byUri.EndpointCount()
	r.RUnlock()

	return count
}

func (r *RouteRegistry) MarshalJSON() ([]byte, error) {
	r.RLock()
	defer r.RUnlock()

	return json.Marshal(r.byUri.ToMap())
}

func (r *RouteRegistry) pruneStaleDroplets() {
	r.Lock()
	r.byUri.EachNodeWithPool(func(t *Trie) {
		t.Pool.PruneEndpoints(r.dropletStaleThreshold)
		t.Snip()
	})
	r.Unlock()
}
