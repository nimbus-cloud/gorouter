package test

import (
	"encoding/json"
	"errors"
	"fmt"
	. "launchpad.net/gocheck"
	"net/http"
	"time"

	"github.com/nimbus-cloud/gorouter/common"
	"github.com/nimbus-cloud/gorouter/route"
	"github.com/cloudfoundry/yagnats"
)

type TestApp struct {
	port       uint16      // app listening port
	rPort      uint16      // router listening port
	urls       []route.Uri // host registered host name
	mbusClient yagnats.NATSClient
	tags       map[string]string
	mux        *http.ServeMux
	stopped    bool
}

func NewTestApp(urls []route.Uri, rPort uint16, mbusClient yagnats.NATSClient, tags map[string]string) *TestApp {
	app := new(TestApp)

	port, _ := common.GrabEphemeralPort()

	app.port = port
	app.rPort = rPort
	app.urls = urls
	app.mbusClient = mbusClient
	app.tags = tags

	app.mux = http.NewServeMux()

	return app
}

func (a *TestApp) AddHandler(path string, handler func(http.ResponseWriter, *http.Request)) {
	a.mux.HandleFunc(path, handler)
}

func (a *TestApp) Urls() []route.Uri {
	return a.urls
}

func (a *TestApp) Endpoint() string {
	return fmt.Sprintf("http://%s:%d/", a.urls[0], a.rPort)
}

func (a *TestApp) Listen() {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", a.port),
		Handler: a.mux,
	}
	a.Register()
	go server.ListenAndServe()
}

func (a *TestApp) RegisterRepeatedly(duration time.Duration) {
	a.stopped = false
	for {
		if a.stopped {
			break
		}
		a.Register()
		time.Sleep(duration)
	}
}

func (a *TestApp) Register() {
	rm := registerMessage{
		Host: "localhost",
		Port: a.port,
		Uris: a.urls,
		Tags: a.tags,
		Dea:  "dea",
		App:  "0",

		PrivateInstanceId: common.GenerateUUID(),
	}

	b, _ := json.Marshal(rm)
	a.mbusClient.Publish("router.register", b)
}

func (a *TestApp) Unregister() {
	rm := registerMessage{
		Host: "localhost",
		Port: a.port,
		Uris: a.urls,
		Tags: nil,
		Dea:  "dea",
		App:  "0",
	}

	b, _ := json.Marshal(rm)
	a.mbusClient.Publish("router.unregister", b)
	a.stopped = true
}

func (a *TestApp) VerifyAppStatus(status int, c *C) {
	check := a.CheckAppStatus(status)
	c.Assert(check, IsNil)
}

func (a *TestApp) CheckAppStatus(status int) error {
	for _, url := range a.urls {
		uri := fmt.Sprintf("http://%s:%d", url, a.rPort)
		resp, err := http.Get(uri)
		if err != nil {
			return err
		}

		if resp.StatusCode != status {
			return errors.New(fmt.Sprintf("expected status code %d, got %d", status, resp.StatusCode))
		}
	}

	return nil
}

type registerMessage struct {
	Host string            `json:"host"`
	Port uint16            `json:"port"`
	Uris []route.Uri       `json:"uris"`
	Tags map[string]string `json:"tags"`
	Dea  string            `json:"dea"`
	App  string            `json:"app"`

	PrivateInstanceId string `json:"private_instance_id"`
}
