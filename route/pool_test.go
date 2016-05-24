package route_test

import (
	"fmt"
	"time"
	"net"

	. "github.com/cloudfoundry/gorouter/route"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)


func endPointInSlice(a *Endpoint, list []*Endpoint) bool {
    for _, b := range list {
        if b == a {
            return true
        }
    }
    return false
}


var _ = Describe("Pool", func() {
	var pool *Pool

	BeforeEach(func() {
		pool = NewPool(2*time.Minute, "", nil)
	})

	Context("Put", func() {
		It("adds endpoints", func() {
			endpoint := &Endpoint{}

			b := pool.Put(endpoint)
			Expect(b).To(BeTrue())
		})

		It("handles duplicate endpoints", func() {
			endpoint := &Endpoint{}

			pool.Put(endpoint)
			b := pool.Put(endpoint)
			Expect(b).To(BeFalse())
		})

		It("handles equivalent (duplicate) endpoints", func() {
			endpoint1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1, "")
			endpoint2 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1, "")

			pool.Put(endpoint1)
			Expect(pool.Put(endpoint2)).To(BeFalse())
		})
	})

	Context("RouteServiceUrl", func() {
		It("returns the route_service_url associated with the pool", func() {
			endpoint := &Endpoint{}
			endpointRS := &Endpoint{RouteServiceUrl: "my-url"}
			b := pool.Put(endpoint)
			Expect(b).To(BeTrue())

			url := pool.RouteServiceUrl()
			Expect(url).To(BeEmpty())

			b = pool.Put(endpointRS)
			Expect(b).To(BeFalse())
			url = pool.RouteServiceUrl()
			Expect(url).To(Equal("my-url"))
		})

		Context("when there are no endpoints in the pool", func() {
			It("returns the empty string", func() {
				url := pool.RouteServiceUrl()
				Expect(url).To(Equal(""))
			})
		})
	})

	Context("Remove", func() {
		It("removes endpoints", func() {
			endpoint := &Endpoint{}
			pool.Put(endpoint)

			b := pool.Remove(endpoint)
			Expect(b).To(BeTrue())
			Expect(pool.IsEmpty()).To(BeTrue())
		})

		It("fails to remove an endpoint that doesn't exist", func() {
			endpoint := &Endpoint{}

			b := pool.Remove(endpoint)
			Expect(b).To(BeFalse())
		})
	})

	Context("IsEmpty", func() {
		It("starts empty", func() {
			Expect(pool.IsEmpty()).To(BeTrue())
		})

		It("not empty after adding an endpoint", func() {
			endpoint := &Endpoint{}
			pool.Put(endpoint)

			Expect(pool.IsEmpty()).To(BeFalse())
		})

		It("is empty after removing everything", func() {
			endpoint := &Endpoint{}
			pool.Put(endpoint)
			pool.Remove(endpoint)

			Expect(pool.IsEmpty()).To(BeTrue())
		})
	})

	Context("PruneEndpoints", func() {
		defaultThreshold := 1 * time.Minute

		Context("when an endpoint has a custom stale time", func() {
			Context("when custom stale threshold is greater than default threshold", func() {
				It("prunes the endpoint", func() {
					customThreshold := int(defaultThreshold.Seconds()) + 20
					e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, customThreshold, "")
					pool.Put(e1)

					updateTime, _ := time.ParseDuration(fmt.Sprintf("%ds", customThreshold-10))
					pool.MarkUpdated(time.Now().Add(-updateTime))

					Expect(pool.IsEmpty()).To(Equal(false))
					pool.PruneEndpoints(defaultThreshold)
					Expect(pool.IsEmpty()).To(Equal(true))
				})
			})

			Context("and it has passed the stale threshold", func() {
				It("prunes the endpoint", func() {
					e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, 20, "")

					pool.Put(e1)
					pool.MarkUpdated(time.Now().Add(-25 * time.Second))

					Expect(pool.IsEmpty()).To(Equal(false))
					pool.PruneEndpoints(defaultThreshold)
					Expect(pool.IsEmpty()).To(Equal(true))
				})
			})

			Context("and it has not passed the stale threshold", func() {
				It("does NOT prune the endpoint", func() {
					e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, 20, "")

					pool.Put(e1)
					pool.MarkUpdated(time.Now())

					Expect(pool.IsEmpty()).To(Equal(false))
					pool.PruneEndpoints(defaultThreshold)
					Expect(pool.IsEmpty()).To(Equal(false))
				})

			})
		})

		Context("when an endpoint does NOT have a custom stale time", func() {
			Context("and it has passed the stale threshold", func() {
				It("prunes the endpoint", func() {
					e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1, "")

					pool.Put(e1)
					pool.MarkUpdated(time.Now().Add(-(defaultThreshold + 1)))

					Expect(pool.IsEmpty()).To(Equal(false))
					pool.PruneEndpoints(defaultThreshold)
					Expect(pool.IsEmpty()).To(Equal(true))
				})
			})

			Context("and it has not passed the stale threshold", func() {
				It("does NOT prune the endpoint", func() {
					e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1, "")

					pool.Put(e1)
					pool.MarkUpdated(time.Now())

					Expect(pool.IsEmpty()).To(Equal(false))
					pool.PruneEndpoints(defaultThreshold)
					Expect(pool.IsEmpty()).To(Equal(false))
				})
			})
		})
	})

	Context("MarkUpdated", func() {
		It("updates all endpoints", func() {
			e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1, "")

			pool.Put(e1)

			threshold := 1 * time.Second
			pool.PruneEndpoints(threshold)
			Expect(pool.IsEmpty()).To(BeFalse())

			pool.MarkUpdated(time.Now())
			pool.PruneEndpoints(threshold)
			Expect(pool.IsEmpty()).To(BeFalse())

			pool.PruneEndpoints(0)
			Expect(pool.IsEmpty()).To(BeTrue())
		})
	})

	Context("Each", func() {
		It("applies a function to each endpoint", func() {
			e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1, "")
			e2 := NewEndpoint("", "5.6.7.8", 1234, "", nil, -1, "")
			pool.Put(e1)
			pool.Put(e2)

			endpoints := make(map[string]*Endpoint)
			pool.Each(func(e *Endpoint) {
				endpoints[e.CanonicalAddr()] = e
			})
			Expect(endpoints).To(HaveLen(2))
			Expect(endpoints[e1.CanonicalAddr()]).To(Equal(e1))
			Expect(endpoints[e2.CanonicalAddr()]).To(Equal(e2))
		})
	})

	Context("PreferredNetwork", func() {
		It("returns only ips from a preferred network", func() {
			_, testnet, _ := net.ParseCIDR("10.1.1.0/24")

			e1 := NewEndpoint("", "10.1.1.1", 5678, "", nil, -1, "")
			e2 := NewEndpoint("", "10.1.1.2", 5678, "", nil, -1, "")
			e3 := NewEndpoint("", "10.1.1.3", 5678, "", nil, -1, "")

			pool = NewPool(2 * time.Minute, "", testnet)
			pool.Put(e1)
			pool.Put(e2)
                        pool.Put(e3)

			pool.Put(NewEndpoint("", "10.1.2.5", 5678, "", nil, -1, ""))
			pool.Put(NewEndpoint("", "10.1.2.5", 1234, "", nil, -1, ""))
			pool.Put(NewEndpoint("", "10.1.2.6", 1234, "", nil, -1, ""))
			pool.Remove(e3)

			iter := pool.Endpoints("")
			r1 := iter.Next()
			r2 := iter.Next()
			r3 := iter.Next()

			array := [] *Endpoint {e1, e2}

			//fmt.Printf("\n")
			//fmt.Printf("r1: %v\n", r1)
			//fmt.Printf("r2: %v\n", r2)
			//fmt.Printf("r3: %v\n", r3)
                        Ω(endPointInSlice(r1, array)).Should(Equal(true))
			Ω(endPointInSlice(r2, array)).Should(Equal(true))
			Ω(endPointInSlice(r3, array)).Should(Equal(true))

		})

		It("returns non preferred networks when preferred aren't available", func() {
                        _, testnet, _ := net.ParseCIDR("10.1.2.0/24")

                        e1 := NewEndpoint("", "10.1.1.1", 5678, "", nil, -1, "")
                        e2 := NewEndpoint("", "10.1.1.2", 5678, "", nil, -1, "")
                        e3 := NewEndpoint("", "10.1.2.3", 5678, "", nil, -1, "")

                        pool = NewPool(2 * time.Minute, "", testnet)
                        pool.Put(e1)
                        pool.Put(e2)
                        pool.Put(e3)
                        pool.Remove(e3)

			array := [] *Endpoint {e1, e2}

                        iter := pool.Endpoints("")
                        r1 := iter.Next()
                        r2 := iter.Next()
                        r3 := iter.Next()

                        Ω(endPointInSlice(r1, array)).Should(Equal(true))
                        Ω(endPointInSlice(r2, array)).Should(Equal(true))
                        Ω(endPointInSlice(r3, array)).Should(Equal(true))

		})

		It("handles all elements of a preferred network being removed", func() {
			_, testnet, _ := net.ParseCIDR("10.1.1.0/24")

			e1 := NewEndpoint("", "10.1.1.1", 5678, "", nil, -1, "")
			e2 := NewEndpoint("", "10.1.1.2", 5678, "", nil, -1, "")
			e3 := NewEndpoint("", "10.1.1.3", 5678, "", nil, -1, "")

			pool = NewPool(2 * time.Minute, "", testnet)
			pool.Put(e1)
			pool.Put(e2)
			pool.Put(e3)
			pool.Remove(e1)
			pool.Remove(e2)
			pool.Remove(e3)

			iter := pool.Endpoints("")
			r1 := iter.Next()
			r2 := iter.Next()
			r3 := iter.Next()

			Ω(r1).Should(BeNil())
			Ω(r2).Should(BeNil())
			Ω(r3).Should(BeNil())

		})

	})


	It("marshals json", func() {
		e := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1, "https://my-rs.com")
		e2 := NewEndpoint("", "5.6.7.8", 5678, "", nil, -1, "")
		pool.Put(e)
		pool.Put(e2)

		json, err := pool.MarshalJSON()
		Expect(err).ToNot(HaveOccurred())

		Expect(string(json)).To(Equal(`[{"address":"1.2.3.4:5678","ttl":-1,"route_service_url":"https://my-rs.com"},{"address":"5.6.7.8:5678","ttl":-1}]`))
	})
})
