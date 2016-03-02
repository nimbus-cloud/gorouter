package proxy_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/events"
	router_http "github.com/cloudfoundry/gorouter/common/http"
	"github.com/cloudfoundry/gorouter/registry"
	"github.com/cloudfoundry/gorouter/route"
	"github.com/cloudfoundry/gorouter/stats"
	"github.com/cloudfoundry/gorouter/test_util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const uuid_regex = `^[[:xdigit:]]{8}(-[[:xdigit:]]{4}){3}-[[:xdigit:]]{12}$`

type connHandler func(*test_util.HttpConn)

type nullVarz struct{}

func (_ nullVarz) MarshalJSON() ([]byte, error)                               { return json.Marshal(nil) }
func (_ nullVarz) ActiveApps() *stats.ActiveApps                              { return stats.NewActiveApps() }
func (_ nullVarz) CaptureBadRequest(*http.Request)                            {}
func (_ nullVarz) CaptureBadGateway(*http.Request)                            {}
func (_ nullVarz) CaptureRoutingRequest(b *route.Endpoint, req *http.Request) {}
func (_ nullVarz) CaptureRoutingResponse(b *route.Endpoint, res *http.Response, t time.Time, d time.Duration) {
}

var _ = Describe("Proxy", func() {
	It("responds to http/1.0 with path", func() {
		ln := registerHandler(r, "test/my_path", func(x *test_util.HttpConn) {
			x.CheckLine("GET /my_path HTTP/1.1")

			x.WriteLines([]string{
				"HTTP/1.1 200 OK",
				"Content-Length: 0",
			})
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		x.WriteLines([]string{
			"GET /my_path HTTP/1.0",
			"Host: test",
		})

		x.CheckLine("HTTP/1.0 200 OK")
	})

	It("responds transparently to a trailing slash versus no trailing slash", func() {
		lnWithoutSlash := registerHandler(r, "test/my%20path/your_path", func(x *test_util.HttpConn) {
			x.CheckLine("GET /my%20path/your_path/ HTTP/1.1")

			x.WriteLines([]string{
				"HTTP/1.1 200 OK",
				"Content-Length: 0",
			})
		})
		defer lnWithoutSlash.Close()

		lnWithSlash := registerHandler(r, "test/another-path/your_path/", func(x *test_util.HttpConn) {
			x.CheckLine("GET /another-path/your_path HTTP/1.1")

			x.WriteLines([]string{
				"HTTP/1.1 200 OK",
				"Content-Length: 0",
			})
		})
		defer lnWithSlash.Close()

		x := dialProxy(proxyServer)
		y := dialProxy(proxyServer)

		x.WriteLines([]string{
			"GET /my%20path/your_path/ HTTP/1.0",
			"Host: test",
		})
		x.CheckLine("HTTP/1.0 200 OK")

		y.WriteLines([]string{
			"GET /another-path/your_path HTTP/1.0",
			"Host: test",
		})
		y.CheckLine("HTTP/1.0 200 OK")
	})

	It("responds to http/1.0 with path/path", func() {
		ln := registerHandler(r, "test/my%20path/your_path", func(x *test_util.HttpConn) {
			x.CheckLine("GET /my%20path/your_path HTTP/1.1")

			x.WriteLines([]string{
				"HTTP/1.1 200 OK",
				"Content-Length: 0",
			})
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		x.WriteLines([]string{
			"GET /my%20path/your_path HTTP/1.0",
			"Host: test",
		})

		x.CheckLine("HTTP/1.0 200 OK")
	})

	It("responds to http/1.0", func() {
		ln := registerHandler(r, "test", func(x *test_util.HttpConn) {
			x.CheckLine("GET / HTTP/1.1")

			x.WriteLines([]string{
				"HTTP/1.1 200 OK",
				"Content-Length: 0",
			})
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		x.WriteLines([]string{
			"GET / HTTP/1.0",
			"Host: test",
		})

		x.CheckLine("HTTP/1.0 200 OK")
	})

	It("Logs a request", func() {
		ln := registerHandler(r, "test", func(x *test_util.HttpConn) {
			req, body := x.ReadRequest()
			Ω(req.Method).Should(Equal("POST"))
			Ω(req.URL.Path).Should(Equal("/"))
			Ω(req.ProtoMajor).Should(Equal(1))
			Ω(req.ProtoMinor).Should(Equal(1))

			Ω(body).Should(Equal("ABCD"))

			rsp := test_util.NewResponse(200)
			out := &bytes.Buffer{}
			out.WriteString("DEFG")
			rsp.Body = ioutil.NopCloser(out)
			x.WriteResponse(rsp)
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		body := &bytes.Buffer{}
		body.WriteString("ABCD")
		req := test_util.NewRequest("POST", "/", ioutil.NopCloser(body))
		req.Host = "test"
		x.WriteRequest(req)

		x.CheckLine("HTTP/1.1 200 OK")

		var payload []byte
		Eventually(func() int {
			accessLogFile.Read(&payload)
			return len(payload)
		}).ShouldNot(BeZero())

		//make sure the record includes all the data
		//since the building of the log record happens throughout the life of the request
		Ω(strings.HasPrefix(string(payload), "test - [")).Should(BeTrue())
		Ω(string(payload)).To(ContainSubstring(`"POST / HTTP/1.1" 200 4 4 "-"`))
		Ω(string(payload)).To(ContainSubstring(`x_forwarded_for:"127.0.0.1" vcap_request_id:`))
		Ω(string(payload)).To(ContainSubstring(`response_time:`))
		Ω(string(payload)).To(ContainSubstring(`app_id:`))
		Ω(payload[len(payload)-1]).To(Equal(byte('\n')))
	})

	It("Logs a request when it exits early", func() {
		x := dialProxy(proxyServer)

		x.WriteLines([]string{
			"GET / HTTP/0.9",
			"Host: test",
		})

		x.CheckLine("HTTP/1.0 400 Bad Request")

		var payload []byte
		Eventually(func() int {
			n, e := accessLogFile.Read(&payload)
			Ω(e).ShouldNot(HaveOccurred())
			return n
		}).ShouldNot(BeZero())

		Ω(string(payload)).To(MatchRegexp("^test.*\n"))
	})

	It("responds to HTTP/1.1", func() {
		ln := registerHandler(r, "test", func(x *test_util.HttpConn) {
			x.CheckLine("GET / HTTP/1.1")

			x.WriteLines([]string{
				"HTTP/1.1 200 OK",
				"Content-Length: 0",
			})
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		x.WriteLines([]string{
			"GET / HTTP/1.1",
			"Host: test",
		})

		x.CheckLine("HTTP/1.1 200 OK")
	})

	It("does not respond to unsupported HTTP versions", func() {
		x := dialProxy(proxyServer)

		x.WriteLines([]string{
			"GET / HTTP/0.9",
			"Host: test",
		})

		x.CheckLine("HTTP/1.0 400 Bad Request")
	})

	It("responds to load balancer check", func() {
		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", "HTTP-Monitor/1.1")
		x.WriteRequest(req)

		_, body := x.ReadResponse()
		Ω(body).To(Equal("ok\n"))
	})

	It("responds to unknown host with 404", func() {
		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "unknown"
		x.WriteRequest(req)

		resp, body := x.ReadResponse()
		Ω(resp.StatusCode).To(Equal(http.StatusNotFound))
		Ω(resp.Header.Get("X-Cf-RouterError")).To(Equal("unknown_route"))
		Ω(body).To(Equal("404 Not Found: Requested route ('unknown') does not exist.\n"))
	})

	It("responds to misbehaving host with 502", func() {
		ln := registerHandler(r, "enfant-terrible", func(x *test_util.HttpConn) {
			x.Close()
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "enfant-terrible"
		x.WriteRequest(req)

		resp, body := x.ReadResponse()
		Ω(resp.StatusCode).To(Equal(http.StatusBadGateway))
		Ω(resp.Header.Get("X-Cf-RouterError")).To(Equal("endpoint_failure"))
		Ω(body).To(Equal("502 Bad Gateway: Registered endpoint failed to handle the request.\n"))
	})

	It("trace headers added on correct TraceKey", func() {
		ln := registerHandler(r, "trace-test", func(x *test_util.HttpConn) {
			_, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "trace-test"
		req.Header.Set(router_http.VcapTraceHeader, "my_trace_key")
		x.WriteRequest(req)

		resp, _ := x.ReadResponse()
		Ω(resp.StatusCode).To(Equal(http.StatusOK))
		Ω(resp.Header.Get(router_http.VcapBackendHeader)).To(Equal(ln.Addr().String()))
		Ω(resp.Header.Get(router_http.CfRouteEndpointHeader)).To(Equal(ln.Addr().String()))
		Ω(resp.Header.Get(router_http.VcapRouterHeader)).To(Equal(conf.Ip))
	})

	It("trace headers not added on incorrect TraceKey", func() {
		ln := registerHandler(r, "trace-test", func(x *test_util.HttpConn) {
			_, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "trace-test"
		req.Header.Set(router_http.VcapTraceHeader, "a_bad_trace_key")
		x.WriteRequest(req)

		resp, _ := x.ReadResponse()
		Ω(resp.Header.Get(router_http.VcapBackendHeader)).To(Equal(""))
		Ω(resp.Header.Get(router_http.CfRouteEndpointHeader)).To(Equal(""))
		Ω(resp.Header.Get(router_http.VcapRouterHeader)).To(Equal(""))
	})

	It("X-Forwarded-For is added", func() {
		done := make(chan bool)

		ln := registerHandler(r, "app", func(x *test_util.HttpConn) {
			req, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()

			done <- req.Header.Get("X-Forwarded-For") == "127.0.0.1"
		})
		defer ln.Close()

		x := dialProxy(proxyServer)
		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "app"
		x.WriteRequest(req)

		var answer bool
		Eventually(done).Should(Receive(&answer))
		Ω(answer).To(BeTrue())

		x.ReadResponse()
	})

	It("X-Forwarded-For is appended", func() {
		done := make(chan bool)

		ln := registerHandler(r, "app", func(x *test_util.HttpConn) {
			req, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()

			done <- req.Header.Get("X-Forwarded-For") == "1.2.3.4, 127.0.0.1"
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "app"
		req.Header.Add("X-Forwarded-For", "1.2.3.4")
		x.WriteRequest(req)

		var answer bool
		Eventually(done).Should(Receive(&answer))
		Ω(answer).To(BeTrue())

		x.ReadResponse()
	})

	It("X-Request-Start is appended", func() {
		done := make(chan string)

		ln := registerHandler(r, "app", func(x *test_util.HttpConn) {
			req, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()

			done <- req.Header.Get("X-Request-Start")
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "app"
		x.WriteRequest(req)

		var answer string
		Eventually(done).Should(Receive(&answer))
		Ω(answer).To(MatchRegexp("^\\d{10}\\d{3}$")) // unix timestamp millis

		x.ReadResponse()
	})

	It("X-Request-Start is not overwritten", func() {
		done := make(chan []string)

		ln := registerHandler(r, "app", func(x *test_util.HttpConn) {
			req, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()

			done <- req.Header[http.CanonicalHeaderKey("X-Request-Start")]
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "app"
		req.Header.Add("X-Request-Start", "") // impl cannot just check for empty string
		req.Header.Add("X-Request-Start", "user-set2")
		x.WriteRequest(req)

		var answer []string
		Eventually(done).Should(Receive(&answer))
		Ω(answer).To(Equal([]string{"", "user-set2"}))

		x.ReadResponse()
	})

	It("X-VcapRequest-Id header is added", func() {
		done := make(chan string)

		ln := registerHandler(r, "app", func(x *test_util.HttpConn) {
			req, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()

			done <- req.Header.Get(router_http.VcapRequestIdHeader)
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "app"
		x.WriteRequest(req)

		var answer string
		Eventually(done).Should(Receive(&answer))
		Ω(answer).To(MatchRegexp(uuid_regex))

		x.ReadResponse()
	})

	It("X-Vcap-Request-Id header is overwritten", func() {
		done := make(chan string)

		ln := registerHandler(r, "app", func(x *test_util.HttpConn) {
			req, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()

			done <- req.Header.Get(router_http.VcapRequestIdHeader)
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "app"
		req.Header.Add(router_http.VcapRequestIdHeader, "A-BOGUS-REQUEST-ID")
		x.WriteRequest(req)

		var answer string
		Eventually(done).Should(Receive(&answer))
		Ω(answer).ToNot(Equal("A-BOGUS-REQUEST-ID"))
		Ω(answer).To(MatchRegexp(uuid_regex))

		x.ReadResponse()
	})

	It("X-CF-InstanceID header is added literally if present in the routing endpoint", func() {
		done := make(chan string)

		ln := registerHandlerWithInstanceId(r, "app", func(x *test_util.HttpConn) {
			req, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()

			done <- req.Header.Get(router_http.CfInstanceIdHeader)
		}, "fake-instance-id")
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "app"
		x.WriteRequest(req)

		var answer string
		Eventually(done).Should(Receive(&answer))
		Ω(answer).To(Equal("fake-instance-id"))

		x.ReadResponse()
	})

	It("emits HTTP start events", func() {
		ln := registerHandlerWithInstanceId(r, "app", func(x *test_util.HttpConn) {
		}, "fake-instance-id")
		defer ln.Close()

		x := dialProxy(proxyServer)

		fakeEmitter := fake.NewFakeEventEmitter("fake")
		dropsonde.InitializeWithEmitter(fakeEmitter)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "app"
		x.WriteRequest(req)

		findStartEvent := func() *events.HttpStart {
			for _, event := range fakeEmitter.GetEvents() {
				startEvent, ok := event.(*events.HttpStart)
				if ok {
					return startEvent
				}
			}

			return nil
		}

		Eventually(findStartEvent).ShouldNot(BeNil())
		Expect(findStartEvent().GetInstanceId()).To(Equal("fake-instance-id"))

		x.ReadResponse()
	})

	It("X-CF-InstanceID header is added with host:port information if NOT present in the routing endpoint", func() {
		done := make(chan string)

		ln := registerHandler(r, "app", func(x *test_util.HttpConn) {
			req, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()

			done <- req.Header.Get(router_http.CfInstanceIdHeader)
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "app"
		x.WriteRequest(req)

		var answer string
		Eventually(done).Should(Receive(&answer))
		Ω(answer).To(MatchRegexp(`^\d+(\.\d+){3}:\d+$`))

		x.ReadResponse()
	})

	It("upgrades for a WebSocket request", func() {
		done := make(chan bool)

		ln := registerHandler(r, "ws", func(x *test_util.HttpConn) {
			req, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			done <- req.Header.Get("Upgrade") == "WebsockeT" &&
				req.Header.Get("Connection") == "UpgradE"

			resp := test_util.NewResponse(http.StatusSwitchingProtocols)
			resp.Header.Set("Upgrade", "WebsockeT")
			resp.Header.Set("Connection", "UpgradE")

			x.WriteResponse(resp)

			x.CheckLine("hello from client")
			x.WriteLine("hello from server")
			x.Close()
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/chat", nil)
		req.Host = "ws"
		req.Header.Set("Upgrade", "WebsockeT")
		req.Header.Set("Connection", "UpgradE")

		x.WriteRequest(req)

		var answer bool
		Eventually(done).Should(Receive(&answer))
		Ω(answer).To(BeTrue())

		resp, _ := x.ReadResponse()
		Ω(resp.StatusCode).To(Equal(http.StatusSwitchingProtocols))
		Ω(resp.Header.Get("Upgrade")).To(Equal("WebsockeT"))
		Ω(resp.Header.Get("Connection")).To(Equal("UpgradE"))

		x.WriteLine("hello from client")
		x.CheckLine("hello from server")

		x.Close()
	})

	It("upgrades for a WebSocket request with comma-separated Connection header", func() {
		done := make(chan bool)

		ln := registerHandler(r, "ws-cs-header", func(x *test_util.HttpConn) {
			req, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			done <- req.Header.Get("Upgrade") == "Websocket" &&
				req.Header.Get("Connection") == "keep-alive, Upgrade"

			resp := test_util.NewResponse(http.StatusSwitchingProtocols)
			resp.Header.Set("Upgrade", "Websocket")
			resp.Header.Set("Connection", "Upgrade")

			x.WriteResponse(resp)

			x.CheckLine("hello from client")
			x.WriteLine("hello from server")
			x.Close()
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/chat", nil)
		req.Host = "ws-cs-header"
		req.Header.Add("Upgrade", "Websocket")
		req.Header.Add("Connection", "keep-alive, Upgrade")

		x.WriteRequest(req)

		var answer bool
		Eventually(done).Should(Receive(&answer))
		Ω(answer).To(BeTrue())

		resp, _ := x.ReadResponse()
		Ω(resp.StatusCode).To(Equal(http.StatusSwitchingProtocols))

		Ω(resp.Header.Get("Upgrade")).To(Equal("Websocket"))
		Ω(resp.Header.Get("Connection")).To(Equal("Upgrade"))

		x.WriteLine("hello from client")
		x.CheckLine("hello from server")

		x.Close()
	})

	It("upgrades for a WebSocket request with multiple Connection headers", func() {
		done := make(chan bool)

		ln := registerHandler(r, "ws-cs-header", func(x *test_util.HttpConn) {
			req, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			done <- req.Header.Get("Upgrade") == "Websocket" &&
				req.Header[http.CanonicalHeaderKey("Connection")][0] == "keep-alive" &&
				req.Header[http.CanonicalHeaderKey("Connection")][1] == "Upgrade"

			resp := test_util.NewResponse(http.StatusSwitchingProtocols)
			resp.Header.Set("Upgrade", "Websocket")
			resp.Header.Set("Connection", "Upgrade")

			x.WriteResponse(resp)

			x.CheckLine("hello from client")
			x.WriteLine("hello from server")
			x.Close()
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/chat", nil)
		req.Host = "ws-cs-header"
		req.Header.Add("Upgrade", "Websocket")
		req.Header.Add("Connection", "keep-alive")
		req.Header.Add("Connection", "Upgrade")

		x.WriteRequest(req)

		var answer bool
		Eventually(done).Should(Receive(&answer))
		Ω(answer).To(BeTrue())

		resp, _ := x.ReadResponse()
		Ω(resp.StatusCode).To(Equal(http.StatusSwitchingProtocols))

		Ω(resp.Header.Get("Upgrade")).To(Equal("Websocket"))
		Ω(resp.Header.Get("Connection")).To(Equal("Upgrade"))

		x.WriteLine("hello from client")
		x.CheckLine("hello from server")

		x.Close()
	})

	It("upgrades a Tcp request", func() {
		ln := registerHandler(r, "tcp-handler", func(x *test_util.HttpConn) {
			x.WriteLine("hello")
			x.CheckLine("hello from client")
			x.WriteLine("hello from server")
			x.Close()
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/chat", nil)
		req.Host = "tcp-handler"
		req.Header.Set("Upgrade", "tcp")

		req.Header.Set("Connection", "UpgradE")

		x.WriteRequest(req)

		x.CheckLine("hello")
		x.WriteLine("hello from client")
		x.CheckLine("hello from server")

		x.Close()
	})

	It("transfers chunked encodings", func() {
		ln := registerHandler(r, "chunk", func(x *test_util.HttpConn) {
			r, w := io.Pipe()

			// Write 3 times on a 100ms interval
			go func() {
				t := time.NewTicker(100 * time.Millisecond)
				defer t.Stop()
				defer w.Close()

				for i := 0; i < 3; i++ {
					<-t.C
					_, err := w.Write([]byte("hello"))
					Ω(err).NotTo(HaveOccurred())
				}
			}()

			_, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusOK)
			resp.TransferEncoding = []string{"chunked"}
			resp.Body = r
			resp.Write(x)
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "chunk"

		err := req.Write(x)
		Ω(err).NotTo(HaveOccurred())

		resp, err := http.ReadResponse(x.Reader, &http.Request{})
		Ω(err).NotTo(HaveOccurred())

		Ω(resp.StatusCode).To(Equal(http.StatusOK))
		Ω(resp.TransferEncoding).To(Equal([]string{"chunked"}))

		// Expect 3 individual reads to complete
		b := make([]byte, 16)
		for i := 0; i < 3; i++ {
			n, err := resp.Body.Read(b[0:])
			if err != nil {
				Ω(err).To(Equal(io.EOF))
			}
			Ω(n).To(Equal(5))
			Ω(string(b[0:n])).To(Equal("hello"))
		}
	})

	It("status no content was no Transfer Encoding response header", func() {
		ln := registerHandler(r, "not-modified", func(x *test_util.HttpConn) {
			_, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			resp := test_util.NewResponse(http.StatusNoContent)
			resp.Header.Set("Connection", "close")
			x.WriteResponse(resp)
			x.Close()
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)

		req.Header.Set("Connection", "close")
		req.Host = "not-modified"
		x.WriteRequest(req)

		resp, _ := x.ReadResponse()
		Ω(resp.StatusCode).To(Equal(http.StatusNoContent))
		Ω(resp.TransferEncoding).To(BeNil())
	})

	It("maintains percent-encoded values in URLs", func() {
		shouldEcho("/abc%2b%2f%25%20%22%3F%5Edef", "/abc%2b%2f%25%20%22%3F%5Edef") // +, /, %, <space>, ", £, ^
	})

	It("does not encode reserved characters in URLs", func() {
		rfc3986_reserved_characters := "!*'();:@&=+$,/?#[]"
		shouldEcho("/"+rfc3986_reserved_characters, "/"+rfc3986_reserved_characters)
	})

	It("maintains encoding of percent-encoded reserved characters", func() {
		encoded_reserved_characters := "%21%27%28%29%3B%3A%40%26%3D%2B%24%2C%2F%3F%23%5B%5D"
		shouldEcho("/"+encoded_reserved_characters, "/"+encoded_reserved_characters)
	})

	It("does not encode unreserved characters in URLs", func() {
		shouldEcho("/abc123_.~def", "/abc123_.~def")
	})

	It("does not percent-encode special characters in URLs (they came in like this, they go out like this)", func() {
		shouldEcho("/abc\"£^def", "/abc\"£^def")
	})

	It("handles requests with encoded query strings", func() {
		queryString := strings.Join([]string{"a=b", url.QueryEscape("b= bc "), url.QueryEscape("c=d&e")}, "&")
		shouldEcho("/test?a=b&b%3D+bc+&c%3Dd%26e", "/test?"+queryString)
	})

	It("request terminates with slow response", func() {
		ln := registerHandler(r, "slow-app", func(x *test_util.HttpConn) {
			_, err := http.ReadRequest(x.Reader)
			Ω(err).NotTo(HaveOccurred())

			time.Sleep(1 * time.Second)
			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "slow-app"

		started := time.Now()
		x.WriteRequest(req)

		resp, _ := x.ReadResponse()
		Ω(resp.StatusCode).To(Equal(http.StatusBadGateway))
		Ω(time.Since(started)).To(BeNumerically("<", time.Duration(800*time.Millisecond)))
	})

	It("proxy detects closed client connection", func() {
		serverResult := make(chan error)
		ln := registerHandler(r, "slow-app", func(x *test_util.HttpConn) {
			x.CheckLine("GET / HTTP/1.1")

			timesToTick := 10

			x.WriteLines([]string{
				"HTTP/1.1 200 OK",
				fmt.Sprintf("Content-Length: %d", timesToTick),
			})

			for i := 0; i < 10; i++ {
				_, err := x.Conn.Write([]byte("x"))
				if err != nil {
					serverResult <- err
					return
				}

				time.Sleep(100 * time.Millisecond)
			}

			serverResult <- nil
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "slow-app"
		x.WriteRequest(req)

		x.Conn.Close()

		var err error
		Eventually(serverResult).Should(Receive(&err))
		Ω(err).NotTo(BeNil())
	})

	Context("respect client keepalives", func() {
		It("closes the connection when told to close", func() {
			ln := registerHandler(r, "remote", func(x *test_util.HttpConn) {
				http.ReadRequest(x.Reader)
				resp := test_util.NewResponse(http.StatusOK)
				resp.Close = true
				x.WriteResponse(resp)
				x.Close()
			})
			defer ln.Close()

			x := dialProxy(proxyServer)

			req := test_util.NewRequest("GET", "/", nil)
			req.Host = "remote"
			req.Close = true
			x.WriteRequest(req)
			resp, _ := x.ReadResponse()
			Ω(resp.StatusCode).To(Equal(http.StatusOK))

			x.WriteRequest(req)
			_, err := http.ReadResponse(x.Reader, &http.Request{})
			Ω(err).Should(HaveOccurred())
		})

		It("keeps the connection alive", func() {
			ln := registerHandler(r, "remote", func(x *test_util.HttpConn) {
				http.ReadRequest(x.Reader)
				resp := test_util.NewResponse(http.StatusOK)
				resp.Close = true
				x.WriteResponse(resp)
				x.Close()
			})
			defer ln.Close()

			x := dialProxy(proxyServer)

			req := test_util.NewRequest("GET", "/", nil)
			req.Host = "remote"
			req.Close = false
			x.WriteRequest(req)
			resp, _ := x.ReadResponse()
			Ω(resp.StatusCode).To(Equal(http.StatusOK))

			x.WriteRequest(req)
			_, err := http.ReadResponse(x.Reader, &http.Request{})
			Ω(err).ShouldNot(HaveOccurred())
		})

	})

	It("disables compression", func() {
		ln := registerHandler(r, "remote", func(x *test_util.HttpConn) {
			request, _ := http.ReadRequest(x.Reader)
			encoding := request.Header["Accept-Encoding"]
			var resp *http.Response
			if len(encoding) != 0 {
				resp = test_util.NewResponse(http.StatusInternalServerError)
			} else {
				resp = test_util.NewResponse(http.StatusOK)
			}
			x.WriteResponse(resp)
			x.Close()
		})
		defer ln.Close()

		x := dialProxy(proxyServer)

		req := test_util.NewRequest("GET", "/", nil)
		req.Host = "remote"
		x.WriteRequest(req)
		resp, _ := x.ReadResponse()
		Ω(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("retries when failed endpoints exist", func() {
		ln := registerHandler(r, "retries", func(x *test_util.HttpConn) {
			x.CheckLine("GET / HTTP/1.1")
			resp := test_util.NewResponse(http.StatusOK)
			x.WriteResponse(resp)
			x.Close()
		})
		defer ln.Close()

		ip, err := net.ResolveTCPAddr("tcp", "localhost:81")
		Ω(err).Should(BeNil())
		registerAddr(r, "retries", ip, "instanceId")

		for i := 0; i < 5; i++ {
			x := dialProxy(proxyServer)

			req := test_util.NewRequest("GET", "/", nil)
			req.Host = "retries"
			x.WriteRequest(req)
			resp, _ := x.ReadResponse()

			Ω(resp.StatusCode).To(Equal(http.StatusOK))
		}
	})
})

func registerAddr(r *registry.RouteRegistry, u string, a net.Addr, instanceId string) {
	h, p, err := net.SplitHostPort(a.String())
	Ω(err).NotTo(HaveOccurred())

	x, err := strconv.Atoi(p)
	Ω(err).NotTo(HaveOccurred())

	r.Register(route.Uri(u), route.NewEndpoint("", h, uint16(x), instanceId, nil, -1))
}

func registerHandler(r *registry.RouteRegistry, u string, h connHandler) net.Listener {
	return registerHandlerWithInstanceId(r, u, h, "")
}

func registerHandlerWithInstanceId(r *registry.RouteRegistry, u string, h connHandler, instanceId string) net.Listener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	Ω(err).NotTo(HaveOccurred())

	go func() {
		var tempDelay time.Duration // how long to sleep on accept failure
		for {
			conn, err := ln.Accept()
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					if tempDelay == 0 {
						tempDelay = 5 * time.Millisecond
					} else {
						tempDelay *= 2
					}
					if max := 1 * time.Second; tempDelay > max {
						tempDelay = max
					}
					fmt.Printf("http: Accept error: %v; retrying in %v\n", err, tempDelay)
					time.Sleep(tempDelay)
					continue
				}
				break
			}
			go func() {
				defer GinkgoRecover()
				h(test_util.NewHttpConn(conn))
			}()
		}
	}()

	registerAddr(r, u, ln.Addr(), instanceId)

	return ln
}

func dialProxy(proxyServer net.Listener) *test_util.HttpConn {
	x, err := net.Dial("tcp", proxyServer.Addr().String())
	Ω(err).NotTo(HaveOccurred())

	return test_util.NewHttpConn(x)
}
