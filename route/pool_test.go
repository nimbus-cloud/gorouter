package route_test

import (
	"fmt"
	"time"

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
		pool = NewPool(2 * time.Minute, nil)
	})

	Context("Put", func() {
		It("adds endpoints", func() {
			endpoint := &Endpoint{}

			b := pool.Put(endpoint)
			Ω(b).Should(BeTrue())
		})

		It("handles duplicate endpoints", func() {
			endpoint := &Endpoint{}

			pool.Put(endpoint)
			b := pool.Put(endpoint)
			Ω(b).Should(BeFalse())
		})

		It("handles equivalent (duplicate) endpoints", func() {
			endpoint1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1)
			endpoint2 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1)

			pool.Put(endpoint1)
			Ω(pool.Put(endpoint2)).Should(BeFalse())
		})
	})

	Context("Remove", func() {
		It("removes endpoints", func() {
			endpoint := &Endpoint{}
			pool.Put(endpoint)

			b := pool.Remove(endpoint)
			Ω(b).Should(BeTrue())
			Ω(pool.IsEmpty()).Should(BeTrue())
		})

		It("fails to remove an endpoint that doesn't exist", func() {
			endpoint := &Endpoint{}

			b := pool.Remove(endpoint)
			Ω(b).Should(BeFalse())
		})
	})

	Context("IsEmpty", func() {
		It("starts empty", func() {
			Ω(pool.IsEmpty()).To(BeTrue())
		})

		It("not empty after adding an endpoint", func() {
			endpoint := &Endpoint{}
			pool.Put(endpoint)

			Ω(pool.IsEmpty()).Should(BeFalse())
		})

		It("is empty after removing everything", func() {
			endpoint := &Endpoint{}
			pool.Put(endpoint)
			pool.Remove(endpoint)

			Ω(pool.IsEmpty()).To(BeTrue())
		})
	})

	Context("PruneEndpoints", func() {
		defaultThreshold := 1 * time.Minute

		Context("when an endpoint has a custom stale time", func() {
			Context("when custom stale threshold is greater than default threshold", func() {
				It("prunes the endpoint", func() {
					customThreshold := int(defaultThreshold.Seconds()) + 20
					e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, customThreshold)
					pool.Put(e1)

					updateTime, _ := time.ParseDuration(fmt.Sprintf("%ds", customThreshold-10))
					pool.MarkUpdated(time.Now().Add(-updateTime))

					Ω(pool.IsEmpty()).To(Equal(false))
					pool.PruneEndpoints(defaultThreshold)
					Ω(pool.IsEmpty()).To(Equal(true))
				})
			})

			Context("and it has passed the stale threshold", func() {
				It("prunes the endpoint", func() {
					e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, 20)

					pool.Put(e1)
					pool.MarkUpdated(time.Now().Add(-25 * time.Second))

					Ω(pool.IsEmpty()).To(Equal(false))
					pool.PruneEndpoints(defaultThreshold)
					Ω(pool.IsEmpty()).To(Equal(true))
				})
			})

			Context("and it has not passed the stale threshold", func() {
				It("does NOT prune the endpoint", func() {
					e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, 20)

					pool.Put(e1)
					pool.MarkUpdated(time.Now())

					Ω(pool.IsEmpty()).To(Equal(false))
					pool.PruneEndpoints(defaultThreshold)
					Ω(pool.IsEmpty()).To(Equal(false))
				})

			})
		})

		Context("when an endpoint does NOT have a custom stale time", func() {
			Context("and it has passed the stale threshold", func() {
				It("prunes the endpoint", func() {
					e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1)

					pool.Put(e1)
					pool.MarkUpdated(time.Now().Add(-(defaultThreshold + 1)))

					Ω(pool.IsEmpty()).To(Equal(false))
					pool.PruneEndpoints(defaultThreshold)
					Ω(pool.IsEmpty()).To(Equal(true))
				})
			})

			Context("and it has not passed the stale threshold", func() {
				It("does NOT prune the endpoint", func() {
					e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1)

					pool.Put(e1)
					pool.MarkUpdated(time.Now())

					Ω(pool.IsEmpty()).To(Equal(false))
					pool.PruneEndpoints(defaultThreshold)
					Ω(pool.IsEmpty()).To(Equal(false))
				})
			})
		})
	})

	Context("MarkUpdated", func() {
		It("updates all endpoints", func() {
			e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1)

			pool.Put(e1)

			threshold := 1 * time.Second
			pool.PruneEndpoints(threshold)
			Ω(pool.IsEmpty()).Should(BeFalse())

			pool.MarkUpdated(time.Now())
			pool.PruneEndpoints(threshold)
			Ω(pool.IsEmpty()).Should(BeFalse())

			pool.PruneEndpoints(0)
			Ω(pool.IsEmpty()).Should(BeTrue())
		})
	})

	Context("Each", func() {
		It("applies a function to each endpoint", func() {
			e1 := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1)
			e2 := NewEndpoint("", "5.6.7.8", 1234, "", nil, -1)
			pool.Put(e1)
			pool.Put(e2)

			endpoints := make(map[string]*Endpoint)
			pool.Each(func(e *Endpoint) {
				endpoints[e.CanonicalAddr()] = e
			})
			Ω(endpoints).Should(HaveLen(2))
			Ω(endpoints[e1.CanonicalAddr()]).Should(Equal(e1))
			Ω(endpoints[e2.CanonicalAddr()]).Should(Equal(e2))
		})
	})

	Context("PreferredNetwork", func() {
		It("returns only ips from a preferred network", func() {
			_, testnet, _ := net.ParseCIDR("10.1.1.0/24")
			
			e1 := NewEndpoint("", "10.1.1.1", 5678, "", nil)
			e2 := NewEndpoint("", "10.1.1.2", 5678, "", nil)
			e3 := NewEndpoint("", "10.1.1.3", 5678, "", nil)
			
			pool = NewPool(2 * time.Minute, testnet)
			pool.Put(e1)
			pool.Put(e2)
                        pool.Put(e3)
			pool.Put(NewEndpoint("", "10.1.2.5", 5678, "", nil))
			pool.Put(NewEndpoint("", "10.1.2.5", 1234, "", nil))
			pool.Put(NewEndpoint("", "10.1.2.6", 1234, "", nil))
			pool.Remove(e3) 
			
			iter := pool.Endpoints("")
			r1 := iter.Next()
			r2 := iter.Next()
			r3 := iter.Next()

			array := [] *Endpoint {e1, e2}

                        Ω(endPointInSlice(r1, array)).Should(Equal(true))
			Ω(endPointInSlice(r2, array)).Should(Equal(true))
			Ω(endPointInSlice(r3, array)).Should(Equal(true))

		})

                It("returns non preferred networks when preferred aren't available", func() {
                        _, testnet, _ := net.ParseCIDR("10.1.2.0/24")

                        e1 := NewEndpoint("", "10.1.1.1", 5678, "", nil)
                        e2 := NewEndpoint("", "10.1.1.2", 5678, "", nil)
                        e3 := NewEndpoint("", "10.1.2.3", 5678, "", nil)

                        pool = NewPool(2 * time.Minute, testnet)
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

			e1 := NewEndpoint("", "10.1.1.1", 5678, "", nil)
			e2 := NewEndpoint("", "10.1.1.2", 5678, "", nil)
			e3 := NewEndpoint("", "10.1.1.3", 5678, "", nil)

			pool = NewPool(2 * time.Minute, testnet)
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
		e := NewEndpoint("", "1.2.3.4", 5678, "", nil, -1)
		pool.Put(e)

		json, err := pool.MarshalJSON()
		Ω(err).ToNot(HaveOccurred())

		Ω(string(json)).To(Equal(`["1.2.3.4:5678"]`))
	})
})
