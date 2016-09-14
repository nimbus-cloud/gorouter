package test_util

import (
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/gorouter/config"

	"time"

	. "github.com/onsi/gomega"
)

func SpecConfig(statusPort, proxyPort uint16, natsPorts ...uint16) *config.Config {
	return generateConfig(statusPort, proxyPort, natsPorts...)
}

func SpecSSLConfig(statusPort, proxyPort, SSLPort uint16, natsPorts ...uint16) *config.Config {
	c := generateConfig(statusPort, proxyPort, natsPorts...)

	c.EnableSSL = true

	_, filename, _, _ := runtime.Caller(0)
	testPath, err := filepath.Abs(filepath.Join(filename, "..", "..", "test", "assets"))
	Expect(err).NotTo(HaveOccurred())

	c.SSLKeyPath = filepath.Join(testPath, "certs", "server.key")
	c.SSLCertPath = filepath.Join(testPath, "certs", "server.pem")
	c.SSLPort = SSLPort
	c.CipherString = "TLS_RSA_WITH_AES_128_CBC_SHA:TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA"

	return c
}

func generateConfig(statusPort, proxyPort uint16, natsPorts ...uint16) *config.Config {
	c := config.DefaultConfig()

	c.Port = proxyPort
	c.Index = 2
	c.TraceKey = "my_trace_key"

	// Hardcode the IP to localhost to avoid leaving the machine while running tests
	c.Ip = "127.0.0.1"

	c.StartResponseDelayInterval = 10 * time.Millisecond
	c.PublishStartMessageIntervalInSeconds = 10
	c.PruneStaleDropletsInterval = 0
	c.DropletStaleThreshold = 0
	c.PublishActiveAppsInterval = 0
	c.Zone = "z1"

	c.EndpointTimeout = 500 * time.Millisecond

	c.Status = config.StatusConfig{
		Port: statusPort,
		User: "user",
		Pass: "pass",
	}

	c.Nats = []config.NatsConfig{}
	for _, natsPort := range natsPorts {
		c.Nats = append(c.Nats, config.NatsConfig{
			Host: "localhost",
			Port: natsPort,
			User: "nats",
			Pass: "nats",
		})
	}

	c.Logging = config.LoggingConfig{
		File:          "/dev/stdout",
		Level:         "debug",
		MetronAddress: "localhost:3457",
		JobName:       "router_test_z1_0",
	}

	c.OAuth = config.OAuthConfig{
		TokenEndpoint:     "uaa.cf.service.internal",
		Port:              8443,
		SkipSSLValidation: true,
	}

	c.RouteServiceSecret = "kCvXxNMB0JO2vinxoru9Hg=="

	return c
}
