// This file was generated by counterfeiter
package fakes

import (
	"bufio"
	"net"
	"net/http"
	"sync"

	"github.com/cloudfoundry/gorouter/proxy"
)

type FakeProxyResponseWriter struct {
	HeaderStub        func() http.Header
	headerMutex       sync.RWMutex
	headerArgsForCall []struct{}
	headerReturns     struct {
		result1 http.Header
	}
	HijackStub        func() (net.Conn, *bufio.ReadWriter, error)
	hijackMutex       sync.RWMutex
	hijackArgsForCall []struct{}
	hijackReturns     struct {
		result1 net.Conn
		result2 *bufio.ReadWriter
		result3 error
	}
	WriteStub        func(b []byte) (int, error)
	writeMutex       sync.RWMutex
	writeArgsForCall []struct {
		b []byte
	}
	writeReturns struct {
		result1 int
		result2 error
	}
	WriteHeaderStub        func(s int)
	writeHeaderMutex       sync.RWMutex
	writeHeaderArgsForCall []struct {
		s int
	}
	DoneStub          func()
	doneMutex         sync.RWMutex
	doneArgsForCall   []struct{}
	FlushStub         func()
	flushMutex        sync.RWMutex
	flushArgsForCall  []struct{}
	StatusStub        func() int
	statusMutex       sync.RWMutex
	statusArgsForCall []struct{}
	statusReturns     struct {
		result1 int
	}
	SizeStub        func() int
	sizeMutex       sync.RWMutex
	sizeArgsForCall []struct{}
	sizeReturns     struct {
		result1 int
	}
}

func (fake *FakeProxyResponseWriter) Header() http.Header {
	fake.headerMutex.Lock()
	fake.headerArgsForCall = append(fake.headerArgsForCall, struct{}{})
	fake.headerMutex.Unlock()
	if fake.HeaderStub != nil {
		return fake.HeaderStub()
	} else {
		return fake.headerReturns.result1
	}
}

func (fake *FakeProxyResponseWriter) HeaderCallCount() int {
	fake.headerMutex.RLock()
	defer fake.headerMutex.RUnlock()
	return len(fake.headerArgsForCall)
}

func (fake *FakeProxyResponseWriter) HeaderReturns(result1 http.Header) {
	fake.HeaderStub = nil
	fake.headerReturns = struct {
		result1 http.Header
	}{result1}
}

func (fake *FakeProxyResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	fake.hijackMutex.Lock()
	fake.hijackArgsForCall = append(fake.hijackArgsForCall, struct{}{})
	fake.hijackMutex.Unlock()
	if fake.HijackStub != nil {
		return fake.HijackStub()
	} else {
		return fake.hijackReturns.result1, fake.hijackReturns.result2, fake.hijackReturns.result3
	}
}

func (fake *FakeProxyResponseWriter) HijackCallCount() int {
	fake.hijackMutex.RLock()
	defer fake.hijackMutex.RUnlock()
	return len(fake.hijackArgsForCall)
}

func (fake *FakeProxyResponseWriter) HijackReturns(result1 net.Conn, result2 *bufio.ReadWriter, result3 error) {
	fake.HijackStub = nil
	fake.hijackReturns = struct {
		result1 net.Conn
		result2 *bufio.ReadWriter
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeProxyResponseWriter) Write(b []byte) (int, error) {
	fake.writeMutex.Lock()
	fake.writeArgsForCall = append(fake.writeArgsForCall, struct {
		b []byte
	}{b})
	fake.writeMutex.Unlock()
	if fake.WriteStub != nil {
		return fake.WriteStub(b)
	} else {
		return fake.writeReturns.result1, fake.writeReturns.result2
	}
}

func (fake *FakeProxyResponseWriter) WriteCallCount() int {
	fake.writeMutex.RLock()
	defer fake.writeMutex.RUnlock()
	return len(fake.writeArgsForCall)
}

func (fake *FakeProxyResponseWriter) WriteArgsForCall(i int) []byte {
	fake.writeMutex.RLock()
	defer fake.writeMutex.RUnlock()
	return fake.writeArgsForCall[i].b
}

func (fake *FakeProxyResponseWriter) WriteReturns(result1 int, result2 error) {
	fake.WriteStub = nil
	fake.writeReturns = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *FakeProxyResponseWriter) WriteHeader(s int) {
	fake.writeHeaderMutex.Lock()
	fake.writeHeaderArgsForCall = append(fake.writeHeaderArgsForCall, struct {
		s int
	}{s})
	fake.writeHeaderMutex.Unlock()
	if fake.WriteHeaderStub != nil {
		fake.WriteHeaderStub(s)
	}
}

func (fake *FakeProxyResponseWriter) WriteHeaderCallCount() int {
	fake.writeHeaderMutex.RLock()
	defer fake.writeHeaderMutex.RUnlock()
	return len(fake.writeHeaderArgsForCall)
}

func (fake *FakeProxyResponseWriter) WriteHeaderArgsForCall(i int) int {
	fake.writeHeaderMutex.RLock()
	defer fake.writeHeaderMutex.RUnlock()
	return fake.writeHeaderArgsForCall[i].s
}

func (fake *FakeProxyResponseWriter) Done() {
	fake.doneMutex.Lock()
	fake.doneArgsForCall = append(fake.doneArgsForCall, struct{}{})
	fake.doneMutex.Unlock()
	if fake.DoneStub != nil {
		fake.DoneStub()
	}
}

func (fake *FakeProxyResponseWriter) DoneCallCount() int {
	fake.doneMutex.RLock()
	defer fake.doneMutex.RUnlock()
	return len(fake.doneArgsForCall)
}

func (fake *FakeProxyResponseWriter) Flush() {
	fake.flushMutex.Lock()
	fake.flushArgsForCall = append(fake.flushArgsForCall, struct{}{})
	fake.flushMutex.Unlock()
	if fake.FlushStub != nil {
		fake.FlushStub()
	}
}

func (fake *FakeProxyResponseWriter) FlushCallCount() int {
	fake.flushMutex.RLock()
	defer fake.flushMutex.RUnlock()
	return len(fake.flushArgsForCall)
}

func (fake *FakeProxyResponseWriter) Status() int {
	fake.statusMutex.Lock()
	fake.statusArgsForCall = append(fake.statusArgsForCall, struct{}{})
	fake.statusMutex.Unlock()
	if fake.StatusStub != nil {
		return fake.StatusStub()
	} else {
		return fake.statusReturns.result1
	}
}

func (fake *FakeProxyResponseWriter) StatusCallCount() int {
	fake.statusMutex.RLock()
	defer fake.statusMutex.RUnlock()
	return len(fake.statusArgsForCall)
}

func (fake *FakeProxyResponseWriter) StatusReturns(result1 int) {
	fake.StatusStub = nil
	fake.statusReturns = struct {
		result1 int
	}{result1}
}

func (fake *FakeProxyResponseWriter) Size() int {
	fake.sizeMutex.Lock()
	fake.sizeArgsForCall = append(fake.sizeArgsForCall, struct{}{})
	fake.sizeMutex.Unlock()
	if fake.SizeStub != nil {
		return fake.SizeStub()
	} else {
		return fake.sizeReturns.result1
	}
}

func (fake *FakeProxyResponseWriter) SizeCallCount() int {
	fake.sizeMutex.RLock()
	defer fake.sizeMutex.RUnlock()
	return len(fake.sizeArgsForCall)
}

func (fake *FakeProxyResponseWriter) SizeReturns(result1 int) {
	fake.SizeStub = nil
	fake.sizeReturns = struct {
		result1 int
	}{result1}
}

var _ proxy.ProxyResponseWriter = new(FakeProxyResponseWriter)
