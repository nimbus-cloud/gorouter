package test

import (
	"fmt"
	"io"
	"net/http"

	"github.com/cloudfoundry/yagnats"

	"github.com/cloudfoundry/gorouter/proxy"
	"github.com/cloudfoundry/gorouter/route"
)

func NewStickyApp(urls []route.Uri, rPort uint16, mbusClient yagnats.NATSConn, tags map[string]string) *TestApp {
	app := NewTestApp(urls, rPort, mbusClient, tags)
	app.AddHandler("/sticky", stickyHandler(app.port))

	return app
}

func stickyHandler(port uint16) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie := &http.Cookie{
			Name:  proxy.StickyCookieKey,
			Value: "xxx",
		}
		http.SetCookie(w, cookie)
		io.WriteString(w, fmt.Sprintf("%d", port))
	}
}
