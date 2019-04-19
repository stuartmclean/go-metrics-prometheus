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
			c.gaugeFromNameAndValue(name, float64(metric.Value()))
		case metrics.Histogram:
			samples := metric.Snapshot().Sample().Values()
			if len(samples) > 0 {
				lastSample := samples[len(samples)-1]
				c.gaugeFromNameAndValue(name, float64(lastSample))
			}
		case metrics.Meter:
			s := metric.Snapshot()
			c.gaugeFromNameAndValue(name, float64(s.Rate1()))
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "mean"), float64(s.RateMean()))

		case metrics.Timer:
			s := metric.Snapshot()
			c.gaugeFromNameAndValue(name, float64(s.Rate1()))

			ps := s.Percentiles([]float64{0.95, 0.99, 0.999})
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "mean"), float64(time.Duration(s.Mean())))
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "min"), float64(s.Min()))
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "max"), float64(s.Max()))
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "p95"), float64(time.Duration(ps[0])))
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "p99"), float64(time.Duration(ps[1])))
			c.gaugeFromNameAndValue(fmt.Sprintf("%s_%s", name, "p999"), float64(time.Duration(ps[2])))
		}
	})
	return nil
}
