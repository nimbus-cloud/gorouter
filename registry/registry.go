package registry

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
	"net"

	"github.com/cloudfoundry/gorouter/config"
	"github.com/cloudfoundry/gorouter/metrics"
	"github.com/cloudfoundry/gorouter/route"
	"github.com/cloudfoundry/yagnats"
	"github.com/pivotal-golang/lager"
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

	logger lager.Logger

	byUri *Trie

	pruneStaleDropletsInterval time.Duration
	dropletStaleThreshold      time.Duration

	messageBus yagnats.NATSConn

	reporter metrics.RouteRegistryReporter

	ticker           *time.Ticker
	timeOfLastUpdate time.Time

	preferredNetwork *net.IPNet
}

func NewRouteRegistry(logger lager.Logger, c *config.Config, mbus yagnats.NATSConn, reporter metrics.RouteRegistryReporter) *RouteRegistry {
	r := &RouteRegistry{}
	r.logger = logger
	r.byUri = NewTrie()

	r.pruneStaleDropletsInterval = c.PruneStaleDropletsInterval
	r.dropletStaleThreshold = c.DropletStaleThreshold

	r.preferredNetwork = c.PreferredNetwork

	r.messageBus = mbus
	r.reporter = reporter
	return r
}

func (r *RouteRegistry) Register(uri route.Uri, endpoint *route.Endpoint) {
	t := time.Now()

	r.reporter.CaptureRegistryMessage(endpoint)

	r.Lock()

	uri = uri.RouteKey()

	pool, found := r.byUri.Find(uri)
	if !found {
		contextPath := parseContextPath(uri)
		pool = route.NewPool(r.dropletStaleThreshold/4, contextPath, r.preferredNetwork)
		r.byUri.Insert(uri, pool)
	}

	pool.Put(endpoint)

	r.timeOfLastUpdate = t
	r.Unlock()
}

func (r *RouteRegistry) Unregister(uri route.Uri, endpoint *route.Endpoint) {
	r.reporter.CaptureRegistryMessage(endpoint)

	r.Lock()

	uri = uri.RouteKey()

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

	uri = uri.RouteKey()
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
					r.logger.Debug("start-pruning-droplets")
					r.pruneStaleDroplets()
					msSinceLastUpdate := uint64(time.Since(r.TimeOfLastUpdate()) / time.Millisecond)
					r.reporter.CaptureRouteStats(r.NumUris(), msSinceLastUpdate)
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

func parseContextPath(uri route.Uri) string {
	contextPath := "/"
	split := strings.SplitN(strings.TrimPrefix(uri.String(), "/"), "/", 2)

	if len(split) > 1 {
		contextPath += split[1]
	}
	return contextPath
}
