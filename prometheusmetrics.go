package prometheusmetrics

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcrowley/go-metrics"
	"strings"
	"time"
)

// PrometheusConfig provides a container with config parameters for the
// Prometheus Exporter

type PrometheusConfig struct {
	namespace     string
	Registry      metrics.Registry // Registry to be exported
	subsystem     string
	promRegistry  prometheus.Registerer //Prometheus registry
	FlushInterval time.Duration         //interval to update prom metrics
	gauges        map[string]prometheus.Gauge
	histograms    map[string]*CustomCollector
	timers        map[string]*CustomCollector
}

type CustomCollector struct {
	metric prometheus.Metric
}

func (c *CustomCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- c.metric
}

func (p *CustomCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.NewGauge(prometheus.GaugeOpts{Name: "Dummy", Help: "Dummy"}).Describe(ch)
}


// NewPrometheusProvider returns a Provider that produces Prometheus metrics.
// Namespace and subsystem are applied to all produced metrics.
func NewPrometheusProvider(r metrics.Registry, namespace string, subsystem string, promRegistry prometheus.Registerer, FlushInterval time.Duration) *PrometheusConfig {
	return &PrometheusConfig{
		namespace:     namespace,
		subsystem:     subsystem,
		Registry:      r,
		promRegistry:  promRegistry,
		FlushInterval: FlushInterval,
		gauges:        make(map[string]prometheus.Gauge),
		histograms:    make(map[string]*CustomCollector),
	}
}

func (c *PrometheusConfig) flattenKey(key string) string {
	key = strings.Replace(key, " ", "_", -1)
	key = strings.Replace(key, ".", "_", -1)
	key = strings.Replace(key, "-", "_", -1)
	key = strings.Replace(key, "=", "_", -1)
	return key
}

func (c *PrometheusConfig) gaugeFromNameAndValue(name string, val float64) {
	key := fmt.Sprintf("%s_%s_%s", c.namespace, c.subsystem, name)
	g, ok := c.gauges[key]
	if !ok {
		g = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: c.flattenKey(c.namespace),
			Subsystem: c.flattenKey(c.subsystem),
			Name:      c.flattenKey(name),
			Help:      name,
		})
		c.promRegistry.MustRegister(g)
		c.gauges[key] = g
	}
	g.Set(val)
}

func (c *PrometheusConfig) outputPrometheusHistogram(name string, i interface{}, buckets []float64) {
	key := fmt.Sprintf("%s_%s_%s", c.namespace, c.subsystem, name)

	h, ok := c.histograms[key]
	if !ok {
		h = &CustomCollector{}
		c.promRegistry.MustRegister(h)
		c.histograms[key] = h
	}

	var ps []float64
	var count uint64
	var sum float64

	switch metric := i.(type) {
	case metrics.Histogram:
		snapshot := metric.Snapshot()
		ps = snapshot.Percentiles(buckets)
		count = uint64(snapshot.Count())
		sum = float64(snapshot.Sum())
	case metrics.Timer:
		snapshot := metric.Snapshot()
		ps = snapshot.Percentiles(buckets)
		count = uint64(snapshot.Count())
		sum = float64(snapshot.Sum())
	default:
		return
	}

	bucketVals := make(map[float64]uint64)

	for ii, bucket := range buckets {
		bucketVals[bucket] = uint64(ps[ii])
	}

	desc := prometheus.NewDesc(
		prometheus.BuildFQName(
			c.flattenKey(c.namespace),
			c.flattenKey(c.subsystem),
			c.flattenKey(name),
		),
		name,
		[]string{},
		map[string]string{},
	)

	constHistogram, err := prometheus.NewConstHistogram(
		desc,
		count,
		sum,
		bucketVals,
	)

	if err == nil {
		h.metric = constHistogram
	}
}

func (c *PrometheusConfig) outputPrometheusTimer(name string, metric metrics.Timer, buckets []float64) {
	key := fmt.Sprintf("%s_%s_%s", c.namespace, c.subsystem, name)

	h, ok := c.histograms[key]
	if !ok {
		h = &CustomCollector{}
		c.promRegistry.MustRegister(h)
		c.histograms[key] = h
	}

	snapshot := metric.Snapshot()
	ps := snapshot.Percentiles(buckets)

	bucketVals := make(map[float64]uint64)

	for ii, bucket := range buckets {
		bucketVals[bucket] = uint64(ps[ii])
	}

	desc := prometheus.NewDesc(
		prometheus.BuildFQName(
			c.flattenKey(c.namespace),
			c.flattenKey(c.subsystem),
			c.flattenKey(name),
		),
		name,
		[]string{},
		map[string]string{},
	)

	constHistogram, err := prometheus.NewConstHistogram(
		desc,
		uint64(snapshot.Count()),
		float64(snapshot.Sum()),
		bucketVals,
	)

	if err == nil {
		h.metric = constHistogram
	}
}

func (c *PrometheusConfig) UpdatePrometheusMetrics() {
	for _ = range time.Tick(c.FlushInterval) {
		c.UpdatePrometheusMetricsOnce()
	}
}

func (c *PrometheusConfig) UpdatePrometheusMetricsOnce() error {
	c.Registry.Each(func(name string, i interface{}) {
		switch metric := i.(type) {
		case metrics.Counter:
			c.gaugeFromNameAndValue(name, float64(metric.Count()))
		case metrics.Gauge:
			c.gaugeFromNameAndValue(name, float64(metric.Value()))
		case metrics.GaugeFloat64:
			c.gaugeFromNameAndValue(name, metric.Value())
		case metrics.Histogram:
			buckets := []float64{0.05, 0.1, 0.25, 0.50, 0.75, 0.9, 0.95, 0.99}
			c.outputPrometheusHistogram(name, metric, buckets)
		case metrics.Meter:
			s := metric.Snapshot()
			c.gaugeFromNameAndValue(name, s.Rate1())
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "mean"), s.RateMean())

		case metrics.Timer:
			s := metric.Snapshot()
			c.gaugeFromNameAndValue(name, s.Rate1())

			buckets := []float64{0.95, 0.99, 0.999}
			c.outputPrometheusHistogram(name, metric, buckets)
			ps := s.Percentiles([]float64{0.95, 0.99, 0.999})
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "mean"), s.Mean())
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "min"), float64(s.Min()))
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "max"), float64(s.Max()))
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "p95"), float64(time.Duration(ps[0])))
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "p99"), float64(time.Duration(ps[1])))
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "p999"), float64(time.Duration(ps[2])))
		}
	})
	return nil
}
