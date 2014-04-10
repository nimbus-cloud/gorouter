package route

import (
	"encoding/json"
	"math/rand"
	"net"
)

type Pool struct {
	endpoints map[string]*Endpoint
	preferred_endpoints map[string]*Endpoint
	preferred_network *net.IPNet
}

func NewPool(pnetwork *net.IPNet) *Pool {
	return &Pool{
		endpoints: make(map[string]*Endpoint),
		preferred_endpoints: make(map[string]*Endpoint),
		preferred_network: pnetwork,
	}
}

func (p *Pool) Add(endpoint *Endpoint) {
	p.endpoints[endpoint.CanonicalAddr()] = endpoint
	
	if p.preferred_network != nil  {
		if p.preferred_network.Contains(net.ParseIP(endpoint.Host)) {
			p.preferred_endpoints[endpoint.CanonicalAddr()] = endpoint
		}
	}
}

func (p *Pool) Remove(endpoint *Endpoint) {
	delete(p.endpoints, endpoint.CanonicalAddr())
	delete(p.preferred_endpoints, endpoint.CanonicalAddr())
}

func (p *Pool) Sample() (*Endpoint, bool) {
	if len(p.endpoints) == 0 {
		return nil, false
	}

	newendpoints := &p.endpoints

	if len(p.preferred_endpoints) != 0 {
		newendpoints = &p.preferred_endpoints
	}
 
	index := rand.Intn(len(*newendpoints))

	ticker := 0
	for _, endpoint := range *newendpoints {
		if ticker == index {
			return endpoint, true
		}

		ticker += 1
	}

	panic("unreachable")
}

func (p *Pool) FindByPrivateInstanceId(id string) (*Endpoint, bool) {
	for _, endpoint := range p.endpoints {
		if endpoint.PrivateInstanceId == id {
			return endpoint, true
		}
	}

	return nil, false
}

func (p *Pool) IsEmpty() bool {
	return len(p.endpoints) == 0
}

func (p *Pool) MarshalJSON() ([]byte, error) {
	addresses := []string{}

	for addr, _ := range p.endpoints {
		addresses = append(addresses, addr)
	}

	return json.Marshal(addresses)
}
