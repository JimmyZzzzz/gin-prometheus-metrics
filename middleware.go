package ginprometheusmetrics

import (
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

var (
	keyUriStr string = "key_uri_request_duration_seconds"
)

//	defined metric
//
// param MetricType counter | gauge | histogram | summary
// param Help introduce this metric
type DefineMetric struct {
	Namespace  string
	Name       string
	Help       string
	MetricType string
	Args       []string
	Buckets    []float64
}

// prometheus with options
type PrometheusOpts struct {
	PushInterval   uint8
	PushGateWayUrl string
	JobName        string
	Instance       string //run instance: example pod-name hostname
	MonitorUri     []string
}

// runtime struct
type PrometheusMiddleware struct {
	opts             PrometheusOpts
	stopSign         chan struct{}
	defineMetrics    map[string]prometheus.Collector
	defineMetricType map[string]string
	logWriter        io.Writer
}

// param ns namespace
func NewPrometheus(ns string, opts PrometheusOpts, metrics []DefineMetric) *PrometheusMiddleware {

	if metrics == nil {
		metrics = make([]DefineMetric, 0)
	}

	//basic metric: duration of key uri
	metrics = append(metrics, DefineMetric{
		Namespace:  ns,
		Name:       keyUriStr,
		Help:       "Duration of key uri request in seconds",
		MetricType: "histogram",
		Args:       []string{"uri", "method", "status"},
		Buckets:    Interval500Mill,
	})

	stopCh := make(chan struct{})
	p := &PrometheusMiddleware{opts: opts, stopSign: stopCh}
	p.defineMetrics = make(map[string]prometheus.Collector)

	//default standout
	p.logWriter = os.Stdout

	for _, m := range metrics {
		collector := newMetric(ns, m)
		p.defineMetrics[m.Name] = collector
		p.defineMetricType[m.Name] = m.MetricType
	}

	return p
}

// gin engine register middleware
func (p *PrometheusMiddleware) Use(e *gin.Engine) {
	e.Use(p.promethuesHandlerFunc())
	go p.pushMetrics()

}

// graceful shutdown
func (p *PrometheusMiddleware) StopPush() {

	p.stopSign <- struct{}{}

}

func (p *PrometheusMiddleware) GetCollector(name string) (cc prometheus.Collector) {

	c, ok := p.defineMetrics[name]

	if ok {
		metricType, _ := p.defineMetricType[name]

		switch metricType {

		case "counter":
			cc = c.(prometheus.CounterVec)

		case "gauge":
			cc = c.(prometheus.GaugeVec)

		case "histogram":
			cc = c.(prometheus.HistogramVec)

		case "summary":
			cc = c.(prometheus.SummaryVec)

		default:
			return
		}

		return
	}

	return

}

func (p *PrometheusMiddleware) SetLogger(w io.Writer) {

	p.logWriter = w

}

func (p *PrometheusMiddleware) promethuesHandlerFunc() gin.HandlerFunc {

	return func(c *gin.Context) {

		if len(p.opts.MonitorUri) > 0 {
			for _, uri := range p.opts.MonitorUri {
				if strings.HasPrefix(c.Request.URL.Path, uri) {
					goto exec
				}
				c.Next()
				return
			}
		} else {
			goto exec
		}

	exec:

		begin := time.Now()
		c.Next()
		latency := time.Since(begin)
		status := c.Writer.Status()

		uriMetric, _ := p.defineMetrics[keyUriStr].(prometheus.HistogramVec)
		uriMetric.WithLabelValues(c.Request.URL.Path, c.Request.Method, strconv.Itoa(status)).Observe(float64(latency.Milliseconds()))

	}
}

func (p *PrometheusMiddleware) pushMetrics() {

	timer := time.NewTicker(time.Duration(p.opts.PushInterval) * time.Second)
	log.SetOutput(p.logWriter)
	for {

		select {

		case <-timer.C:
			timestamp := time.Now().Format(time.DateTime)
			pusher := push.New(p.opts.PushGateWayUrl, p.opts.JobName)

			for _, metric := range p.defineMetrics {
				pusher.Collector(metric)
			}

			err := pusher.Grouping("instance", p.opts.Instance).Push()

			if err != nil {
				log.Printf("Could not push to Pushgateway: %v", err)
			} else {
				log.Printf("Metrics pushed successfully with timestamp: %s", timestamp)
			}

		case <-p.stopSign:
			return

		}

	}

}
