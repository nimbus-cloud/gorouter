package route

import (
	"encoding/json"
	"math/rand"
	"sync"
	"time"
	"net"
)

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

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
	endpoint *Endpoint
	index    int
	preferred_index int
	updated  time.Time
	failedAt *time.Time
}

type Pool struct {
	lock      sync.Mutex
	endpoints []*endpointElem
	preferred_endpoints []*endpointElem
	index     map[string]*endpointElem

	preferred_network *net.IPNet

	retryAfterFailure time.Duration
	nextIdx           int
	nextPreferredIdx  int
}

func NewPool(retryAfterFailure time.Duration, pnetwork *net.IPNet) *Pool {
	return &Pool{
		endpoints:         make([]*endpointElem, 0, 1),
		preferred_endpoints: make([]*endpointElem, 0, 1),
		preferred_network: pnetwork,
		index:             make(map[string]*endpointElem),
		retryAfterFailure: retryAfterFailure,
		nextIdx:           -1,
		nextPreferredIdx:  -1,
	}
}

func (p *Pool) Put(endpoint *Endpoint) bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	e, found := p.index[endpoint.CanonicalAddr()]
	if found {
		if e.endpoint == endpoint {
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
			endpoint: endpoint,
			index:    len(p.endpoints),
			preferred_index: -1,
		}
 
		p.endpoints = append(p.endpoints, e)
		
		if p.preferred_network != nil  {
			if p.preferred_network.Contains(net.ParseIP(endpoint.Host)) {
			    e.preferred_index = len(p.preferred_endpoints)
				p.preferred_endpoints = append(p.preferred_endpoints, e)
			}
		}

		p.index[endpoint.CanonicalAddr()] = e
		p.index[endpoint.PrivateInstanceId] = e
	}

	e.updated = time.Now()

	return !found
}

func (p *Pool) PruneEndpoints(defaultThreshold time.Duration) {
	p.lock.Lock()

	last := len(p.endpoints)
	now := time.Now()

	for i := 0; i < last; {
		e := p.endpoints[i]

		staleTime := now.Add(-defaultThreshold)
		if e.endpoint.staleThreshold > 0 && e.endpoint.staleThreshold < defaultThreshold {
			staleTime = now.Add(-e.endpoint.staleThreshold)
		}

		if e.updated.Before(staleTime) {
			p.removeEndpoint(e)
			last--
		} else {
			i++
		}
	}

	p.lock.Unlock()
}

func (p *Pool) Remove(endpoint *Endpoint) bool {
	var e *endpointElem

	p.lock.Lock()
	l := len(p.endpoints)
	if l > 0 {
		e = p.index[endpoint.CanonicalAddr()]
		if e != nil {
			p.removeEndpoint(e)
		}
	}
	p.lock.Unlock()

	return e != nil
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

	if p.nextIdx == -1 {
		p.nextIdx = random.Intn(last)
	} else if p.nextIdx >= last {
		p.nextIdx = 0
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
	addresses := make([]string, 0, len(p.endpoints))
	for _, e := range p.endpoints {
		addresses = append(addresses, e.endpoint.addr)
	}
	p.lock.Unlock()

	return json.Marshal(addresses)
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
