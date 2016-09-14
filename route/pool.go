package route

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"code.cloudfoundry.org/routing-api/models"
)

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

type Endpoint struct {
	ApplicationId        string
	addr                 string
	Tags                 map[string]string
	PrivateInstanceId    string
	staleThreshold       time.Duration
	RouteServiceUrl      string
	PrivateInstanceIndex string
	ModificationTag      models.ModificationTag
}

type EndpointIterator interface {
	Next() *Endpoint
	EndpointFailed()
}

type endpointIterator struct {
	pool *Pool

	initialEndpoint string
	lastEndpoint    *Endpoint
}

type endpointElem struct {
	endpoint        *Endpoint
	index           int
	preferred_index int
	updated         time.Time
	failedAt        *time.Time
}

type Pool struct {
	lock                sync.Mutex
	endpoints           []*endpointElem
	preferred_endpoints []*endpointElem
	index               map[string]*endpointElem

	contextPath     string
	routeServiceUrl string

	preferred_network *net.IPNet

	retryAfterFailure time.Duration
	nextIdx           int
	nextPreferredIdx  int
}

func NewEndpoint(appId, host string, port uint16, privateInstanceId string, privateInstanceIndex string,
	tags map[string]string, staleThresholdInSeconds int, routeServiceUrl string, modificationTag models.ModificationTag) *Endpoint {
	return &Endpoint{
		ApplicationId:        appId,
		addr:                 fmt.Sprintf("%s:%d", host, port),
		Tags:                 tags,
		PrivateInstanceId:    privateInstanceId,
		PrivateInstanceIndex: privateInstanceIndex,
		staleThreshold:       time.Duration(staleThresholdInSeconds) * time.Second,
		RouteServiceUrl:      routeServiceUrl,
		ModificationTag:      modificationTag,
	}
}

func NewPool(retryAfterFailure time.Duration, contextPath string, pnetwork *net.IPNet) *Pool {
	return &Pool{
		endpoints:           make([]*endpointElem, 0, 1),
		preferred_endpoints: make([]*endpointElem, 0, 1),
		preferred_network:   pnetwork,
		index:               make(map[string]*endpointElem),
		retryAfterFailure:   retryAfterFailure,
		nextIdx:             -1,
		nextPreferredIdx:    -1,
		contextPath:         contextPath,
	}
}

func (p *Pool) ContextPath() string {
	return p.contextPath
}

// Returns true if endpoint was added or updated, false otherwise
func (p *Pool) Put(endpoint *Endpoint) bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	e, found := p.index[endpoint.CanonicalAddr()]
	if found {
		if e.endpoint == endpoint {
			return false
		}

		// check modification tag
		if !e.endpoint.ModificationTag.SucceededBy(&endpoint.ModificationTag) {
			return false
		}

		oldEndpoint := e.endpoint
		e.endpoint = endpoint

		if oldEndpoint.PrivateInstanceId != endpoint.PrivateInstanceId {
			delete(p.index, oldEndpoint.PrivateInstanceId)
			p.index[endpoint.PrivateInstanceId] = e
		}
	} else {
		e = &endpointElem{
			endpoint:        endpoint,
			index:           len(p.endpoints),
			preferred_index: -1,
		}

		p.endpoints = append(p.endpoints, e)

		if p.preferred_network != nil {
			if p.preferred_network.Contains(net.ParseIP(endpoint.Host)) {
				e.preferred_index = len(p.preferred_endpoints)
				p.preferred_endpoints = append(p.preferred_endpoints, e)
			}
		}

		p.index[endpoint.CanonicalAddr()] = e
		p.index[endpoint.PrivateInstanceId] = e
	}

	e.updated = time.Now()

	return true
}

func (p *Pool) RouteServiceUrl() string {
	p.lock.Lock()
	defer p.lock.Unlock()

	if len(p.endpoints) > 0 {
		endpt := p.endpoints[0]
		return endpt.endpoint.RouteServiceUrl
	} else {
		return ""
	}
}

func (p *Pool) PruneEndpoints(defaultThreshold time.Duration) []*Endpoint {
	p.lock.Lock()

	last := len(p.endpoints)
	now := time.Now()

	prunedEndpoints := []*Endpoint{}

	for i := 0; i < last; {
		e := p.endpoints[i]

		staleTime := now.Add(-defaultThreshold)
		if e.endpoint.staleThreshold > 0 && e.endpoint.staleThreshold < defaultThreshold {
			staleTime = now.Add(-e.endpoint.staleThreshold)
		}

		if e.updated.Before(staleTime) {
			p.removeEndpoint(e)
			prunedEndpoints = append(prunedEndpoints, e.endpoint)
			last--
		} else {
			i++
		}
	}

	p.lock.Unlock()
	return prunedEndpoints
}

// Returns true if the endpoint was removed from the Pool, false otherwise.
func (p *Pool) Remove(endpoint *Endpoint) bool {
	var e *endpointElem

	p.lock.Lock()
	defer p.lock.Unlock()
	l := len(p.endpoints)
	if l > 0 {
		e = p.index[endpoint.CanonicalAddr()]
		if e != nil && e.endpoint.modificationTagSameOrNewer(endpoint) {
			p.removeEndpoint(e)
			return true
		}
	}

	return false
}

func (p *Pool) removeEndpoint(e *endpointElem) {
	i := e.index
	es := p.endpoints
	last := len(es)
	// re-ordering delete
	es[last-1], es[i], es = nil, es[last-1], es[:last-1]
	if i < last-1 {
		es[i].index = i
	}
	p.endpoints = es

	pi := e.preferred_index
	if e.preferred_index != -1 {
		es := p.preferred_endpoints
		last := len(es)
		// re-ordering delete
		es[last-1], es[pi], es = nil, es[last-1], es[:last-1]
		if pi < last-1 {
			es[pi].preferred_index = pi
		}
		p.preferred_endpoints = es
	}

	delete(p.index, e.endpoint.CanonicalAddr())
	delete(p.index, e.endpoint.PrivateInstanceId)
}

func (p *Pool) Endpoints(initial string) EndpointIterator {
	return newEndpointIterator(p, initial)
}

func (p *Pool) next() *Endpoint {
	p.lock.Lock()
	defer p.lock.Unlock()

	endpoints := p.endpoints
	nextIdx := &p.nextIdx

	if len(p.preferred_endpoints) != 0 {
		endpoints = p.preferred_endpoints
		nextIdx = &p.nextPreferredIdx
	}

	last := len(endpoints)
	if last == 0 {
		return nil
	}

	if *nextIdx == -1 {
		*nextIdx = random.Intn(last)
	} else if *nextIdx >= last {
		*nextIdx = 0
	}

	startIdx := *nextIdx
	curIdx := startIdx
	for {
		e := endpoints[curIdx]

		curIdx++
		if curIdx == last {
			curIdx = 0
		}

		if e.failedAt != nil {
			curTime := time.Now()
			if curTime.Sub(*e.failedAt) > p.retryAfterFailure {
				// exipired failure window
				e.failedAt = nil
			}
		}

		if e.failedAt == nil {
			*nextIdx = curIdx
			return e.endpoint
		}

		if curIdx == startIdx {
			// all endpoints are marked failed so reset everything to available
			for _, e2 := range p.endpoints {
				e2.failedAt = nil
			}
		}
	}
}

func (p *Pool) findById(id string) *Endpoint {
	var endpoint *Endpoint
	p.lock.Lock()
	e := p.index[id]
	if e != nil {
		endpoint = e.endpoint
	}
	p.lock.Unlock()

	return endpoint
}

func (p *Pool) IsEmpty() bool {
	p.lock.Lock()
	l := len(p.endpoints)
	p.lock.Unlock()

	return l == 0
}

func (p *Pool) MarkUpdated(t time.Time) {
	p.lock.Lock()
	for _, e := range p.endpoints {
		e.updated = t
	}
	p.lock.Unlock()
}

func (p *Pool) endpointFailed(endpoint *Endpoint) {
	p.lock.Lock()
	e := p.index[endpoint.CanonicalAddr()]
	if e != nil {
		e.failed()
	}
	p.lock.Unlock()
}

func (p *Pool) Each(f func(endpoint *Endpoint)) {
	p.lock.Lock()
	for _, e := range p.endpoints {
		f(e.endpoint)
	}
	p.lock.Unlock()
}

func (p *Pool) MarshalJSON() ([]byte, error) {
	p.lock.Lock()
	endpoints := make([]Endpoint, 0, len(p.endpoints))
	for _, e := range p.endpoints {
		endpoints = append(endpoints, *e.endpoint)
	}
	p.lock.Unlock()

	return json.Marshal(endpoints)
}

func newEndpointIterator(p *Pool, initial string) EndpointIterator {
	return &endpointIterator{
		pool:            p,
		initialEndpoint: initial,
	}
}

func (i *endpointIterator) Next() *Endpoint {
	var e *Endpoint
	if i.initialEndpoint != "" {
		e = i.pool.findById(i.initialEndpoint)
		i.initialEndpoint = ""
	}

	if e == nil {
		e = i.pool.next()
	}

	i.lastEndpoint = e

	return e
}

func (i *endpointIterator) EndpointFailed() {
	if i.lastEndpoint != nil {
		i.pool.endpointFailed(i.lastEndpoint)
	}
}

func (e *endpointElem) failed() {
	t := time.Now()
	e.failedAt = &t
}

func (e *Endpoint) MarshalJSON() ([]byte, error) {
	var jsonObj struct {
		Address         string `json:"address"`
		TTL             int    `json:"ttl"`
		RouteServiceUrl string `json:"route_service_url,omitempty"`
	}

	jsonObj.Address = e.addr
	jsonObj.RouteServiceUrl = e.RouteServiceUrl
	jsonObj.TTL = int(e.staleThreshold.Seconds())
	return json.Marshal(jsonObj)
}

func (e *Endpoint) CanonicalAddr() string {
	return e.addr
}

func (rm *Endpoint) Component() string {
	return rm.Tags["component"]
}

func (e *Endpoint) ToLogData() interface{} {
	return struct {
		ApplicationId   string
		Addr            string
		Tags            map[string]string
		RouteServiceUrl string
	}{
		e.ApplicationId,
		e.addr,
		e.Tags,
		e.RouteServiceUrl,
	}
}
func (e *Endpoint) modificationTagSameOrNewer(other *Endpoint) bool {
	return e.ModificationTag == other.ModificationTag || e.ModificationTag.SucceededBy(&other.ModificationTag)
}
