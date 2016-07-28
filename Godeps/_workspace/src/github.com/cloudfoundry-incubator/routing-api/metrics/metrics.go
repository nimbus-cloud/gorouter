package metrics

import (
	"os"
	"time"

	"sync/atomic"

	"github.com/cloudfoundry-incubator/routing-api/db"
	"github.com/cloudfoundry/storeadapter"
)

const (
	TotalHttpSubscriptions = "total_http_subscriptions"
	TotalHttpRoutes        = "total_http_routes"
	TotalTcpSubscriptions  = "total_tcp_subscriptions"
	TotalTcpRoutes         = "total_tcp_routes"
	TotalTokenErrors       = "total_token_errors"
	KeyRefreshEvents       = "key_refresh_events"
)

type PartialStatsdClient interface {
	GaugeDelta(stat string, value int64, rate float32) error
	Gauge(stat string, value int64, rate float32) error
}

type MetricsReporter struct {
	db       db.DB
	stats    PartialStatsdClient
	ticker   *time.Ticker
	doneChan chan bool
}

var (
	totalTokenErrors          int64
	totalKeyRefreshEventCount int64
)

func NewMetricsReporter(database db.DB, stats PartialStatsdClient, ticker *time.Ticker) *MetricsReporter {
	return &MetricsReporter{db: database, stats: stats, ticker: ticker}
}

func (r *MetricsReporter) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	httpEventChan, _, httpErrChan := r.db.WatchRouteChanges(db.HTTP_ROUTE_BASE_KEY)
	tcpEventChan, _, tcpErrChan := r.db.WatchRouteChanges(db.TCP_MAPPING_BASE_KEY)
	close(ready)
	ready = nil

	r.stats.Gauge(TotalHttpSubscriptions, 0, 1.0)
	r.stats.Gauge(TotalTcpSubscriptions, 0, 1.0)

	for {
		select {
		case event := <-httpEventChan:
			statsDelta := getStatsEventType(event)
			r.stats.GaugeDelta(TotalHttpRoutes, statsDelta, 1.0)
		case event := <-tcpEventChan:
			statsDelta := getStatsEventType(event)
			r.stats.GaugeDelta(TotalTcpRoutes, statsDelta, 1.0)
		case <-r.ticker.C:
			r.stats.Gauge(TotalHttpRoutes, r.getTotalRoutes(), 1.0)
			r.stats.GaugeDelta(TotalHttpSubscriptions, 0, 1.0)
			r.stats.Gauge(TotalTcpRoutes, r.getTotalTcpRoutes(), 1.0)
			r.stats.GaugeDelta(TotalTcpSubscriptions, 0, 1.0)

			r.stats.Gauge(TotalTokenErrors, GetTokenErrors(), 1.0)
			r.stats.Gauge(KeyRefreshEvents, GetKeyVerificationRefreshCount(), 1.0)
		case <-signals:
			return nil
		case err := <-httpErrChan:
			return err
		case err := <-tcpErrChan:
			return err
		}
	}
}

func (r MetricsReporter) getTotalRoutes() int64 {
	routes, _ := r.db.ReadRoutes()
	return int64(len(routes))
}

func (r MetricsReporter) getTotalTcpRoutes() int64 {
	routes, _ := r.db.ReadTcpRouteMappings()
	return int64(len(routes))
}

func getStatsEventType(event storeadapter.WatchEvent) int64 {
	if event.PrevNode == nil && event.Type == storeadapter.UpdateEvent {
		return 1
	} else if event.Type == storeadapter.ExpireEvent || event.Type == storeadapter.DeleteEvent {
		return -1
	} else {
		return 0
	}
}

func GetTokenErrors() int64 {
	return atomic.LoadInt64(&totalTokenErrors)
}

func IncrementTokenError() {
	atomic.AddInt64(&totalTokenErrors, 1)
}

func GetKeyVerificationRefreshCount() int64 {
	return atomic.LoadInt64(&totalKeyRefreshEventCount)
}

func IncrementKeyVerificationRefreshCount() {
	atomic.AddInt64(&totalKeyRefreshEventCount, 1)
}
