package common_test

import (
	. "github.com/nimbus-cloud/gorouter/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Healthz", func() {
	It("has a Value", func() {
		healthz := &Healthz{}
		ok := healthz.Value()
		Ω(ok).Should(Equal("ok"))
	})
})
