package main

import (
	"flag"
	"os"

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

	errChan := router.NewRouter(c).Run()

	select {
	case err := <-errChan:
		log.Errorf("Error occurred:", err.Error())
		os.Exit(1)
	}
}
