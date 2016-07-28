// This file was generated by counterfeiter
package fakes

import (
	"sync"

	"github.com/cloudfoundry/gorouter/route"
)

type FakeEndpointIterator struct {
	NextStub        func() *route.Endpoint
	nextMutex       sync.RWMutex
	nextArgsForCall []struct{}
	nextReturns     struct {
		result1 *route.Endpoint
	}
	EndpointFailedStub        func()
	endpointFailedMutex       sync.RWMutex
	endpointFailedArgsForCall []struct{}
}

func (fake *FakeEndpointIterator) Next() *route.Endpoint {
	fake.nextMutex.Lock()
	fake.nextArgsForCall = append(fake.nextArgsForCall, struct{}{})
	fake.nextMutex.Unlock()
	if fake.NextStub != nil {
		return fake.NextStub()
	} else {
		return fake.nextReturns.result1
	}
}

func (fake *FakeEndpointIterator) NextCallCount() int {
	fake.nextMutex.RLock()
	defer fake.nextMutex.RUnlock()
	return len(fake.nextArgsForCall)
}

func (fake *FakeEndpointIterator) NextReturns(result1 *route.Endpoint) {
	fake.NextStub = nil
	fake.nextReturns = struct {
		result1 *route.Endpoint
	}{result1}
}

func (fake *FakeEndpointIterator) EndpointFailed() {
	fake.endpointFailedMutex.Lock()
	fake.endpointFailedArgsForCall = append(fake.endpointFailedArgsForCall, struct{}{})
	fake.endpointFailedMutex.Unlock()
	if fake.EndpointFailedStub != nil {
		fake.EndpointFailedStub()
	}
}

func (fake *FakeEndpointIterator) EndpointFailedCallCount() int {
	fake.endpointFailedMutex.RLock()
	defer fake.endpointFailedMutex.RUnlock()
	return len(fake.endpointFailedArgsForCall)
}

var _ route.EndpointIterator = new(FakeEndpointIterator)
