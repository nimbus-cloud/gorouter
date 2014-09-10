package test_util

import (
	vcap "github.com/nimbus-cloud/gorouter/common"
	. "github.com/onsi/gomega"
)

func NextAvailPort() uint16 {
	port, err := vcap.GrabEphemeralPort()
	Ω(err).ShouldNot(HaveOccurred())

	return port
}
