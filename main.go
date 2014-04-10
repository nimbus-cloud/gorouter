package main

import (
	"flag"

	"github.com/nimbus-cloud/gorouter/config"
	"github.com/nimbus-cloud/gorouter/log"
	"github.com/nimbus-cloud/gorouter/router"
)

var configFile string

func init() {
	flag.StringVar(&configFile, "c", "", "Configuration File")

	flag.Parse()
}

func main() {
	c := config.DefaultConfig()
	if configFile != "" {
		c = config.InitConfigFromFile(configFile)
	}

	log.SetupLoggerFromConfig(c)

	router.NewRouter(c).Run()

	select {}
}
