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
type prometheusMiddleware struct {
	opts             PrometheusOpts
	stopSign         chan struct{}
	defineMetrics    map[string]prometheus.Collector
	defineMetricType map[string]string
	logWriter        io.Writer
}

// param ns namespace
func NewPrometheus(ns string, opts PrometheusOpts, metrics []DefineMetric) *prometheusMiddleware {

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
	p := &prometheusMiddleware{opts: opts, stopSign: stopCh}
	p.defineMetrics = make(map[string]prometheus.Collector)
	p.defineMetricType = make(map[string]string)

	//default standout
	p.logWriter = os.Stdout

	if len(metrics) > 0 {

		for _, m := range metrics {
			collector := newMetric(ns, m)
			p.defineMetrics[m.Name] = collector
			p.defineMetricType[m.Name] = m.MetricType
		}

	}

	return p
}

// gin engine register middleware
func (p *prometheusMiddleware) Use(e *gin.Engine) {
	e.Use(p.promethuesHandlerFunc())
	go p.pushMetrics()

}

// graceful shutdown
func (p *prometheusMiddleware) StopPush() {

	p.stopSign <- struct{}{}

}

// return value on demand
func (p *prometheusMiddleware) GetCollector(name string) (c1 prometheus.Counter, c2 prometheus.Gauge, c3 prometheus.Histogram, c4 prometheus.Summary) {

	c, ok := p.defineMetrics[name]

	if ok {
		metricType, _ := p.defineMetricType[name]

		switch metricType {

		case "counter":
			c1 = c.(prometheus.Counter)

		case "gauge":
			c2 = c.(prometheus.Gauge)

		case "histogram":
			c3 = c.(prometheus.Histogram)

		case "summary":
			c4 = c.(prometheus.Summary)

		default:
			return
		}

		return
	}

	return

}

func (p *prometheusMiddleware) SetLogger(w io.Writer) {

	p.logWriter = w

}

func (p *prometheusMiddleware) promethuesHandlerFunc() gin.HandlerFunc {

	return func(c *gin.Context) {

		if len(p.opts.MonitorUri) > 0 {
			for _, uri := range p.opts.MonitorUri {
				if strings.HasPrefix(c.Request.URL.Path, uri) {
					goto exec
				}
			}
			c.Next()
			return
		}
	exec:

		begin := time.Now()
		c.Next()
		latency := time.Since(begin)
		status := c.Writer.Status()

		uriMetric, _ := p.defineMetrics[keyUriStr].(*prometheus.HistogramVec)
		uriMetric.WithLabelValues(c.Request.URL.Path, c.Request.Method, strconv.Itoa(status)).Observe(float64(latency.Milliseconds()))

	}
}

func (p *prometheusMiddleware) pushMetrics() {

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
