package prometheusmetrics

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rcrowley/go-metrics"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPrometheusRegistration(t *testing.T) {
	defaultRegistry := prometheus.DefaultRegisterer
	pClient := NewPrometheusProvider(metrics.DefaultRegistry, "test", "subsys", defaultRegistry, 1*time.Second)
	if pClient.promRegistry != defaultRegistry {
		t.Fatalf("Failed to pass prometheus registry to go-metrics provider")
	}
}

func TestUpdatePrometheusMetricsOnce(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	metricsRegistry.Register("counter", metrics.NewCounter())
	pClient.UpdatePrometheusMetricsOnce()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "test",
		Subsystem: "subsys",
		Name:      "counter",
		Help:      "counter",
	})
	err := prometheusRegistry.Register(gauge)
	if err == nil {
		t.Fatalf("Go-metrics registry didn't get registered to prometheus registry")
	}

}

func TestUpdatePrometheusMetrics(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	metricsRegistry.Register("counter", metrics.NewCounter())
	go pClient.UpdatePrometheusMetrics()
	time.Sleep(2 * time.Second)
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "test",
		Subsystem: "subsys",
		Name:      "counter",
		Help:      "counter",
	})
	err := prometheusRegistry.Register(gauge)
	if err == nil {
		t.Fatalf("Go-metrics registry didn't get registered to prometheus registry")
	}

}

func TestPrometheusCounterGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	cntr := metrics.NewCounter()
	metricsRegistry.Register("counter", cntr)
	cntr.Inc(2)
	go pClient.UpdatePrometheusMetrics()
	cntr.Inc(13)
	time.Sleep(5 * time.Second)
	metrics, _ := prometheusRegistry.Gather()
	serialized := fmt.Sprint(metrics[0])
	expected := fmt.Sprintf("name:\"test_subsys_counter\" help:\"counter\" type:GAUGE metric:<gauge:<value:%d > > ", cntr.Count())
	if serialized != expected {
		t.Fatalf("Go-metrics value and prometheus metrics value do not match")
	}
}

func TestPrometheusGaugeGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	gm := metrics.NewGauge()
	metricsRegistry.Register("gauge", gm)
	gm.Update(2)
	go pClient.UpdatePrometheusMetrics()
	gm.Update(13)
	time.Sleep(5 * time.Second)
	metrics, _ := prometheusRegistry.Gather()
	if len(metrics) == 0 {
		t.Fatalf("prometheus was unable to register the metric")
	}
	serialized := fmt.Sprint(metrics[0])
	expected := fmt.Sprintf("name:\"test_subsys_gauge\" help:\"gauge\" type:GAUGE metric:<gauge:<value:%d > > ", gm.Value())
	if serialized != expected {
		t.Fatalf("Go-metrics value and prometheus metrics value do not match")
	}
}

func TestPrometheusMeterGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	gm := metrics.NewMeter()
	metricsRegistry.Register("meter", gm)
	gm.Mark(2)
	go pClient.UpdatePrometheusMetrics()
	gm.Mark(13)
	time.Sleep(5 * time.Second)
	metrics, _ := prometheusRegistry.Gather()
	if len(metrics) == 0 {
		t.Fatalf("prometheus was unable to register the metric")
	}
	serialized := fmt.Sprint(metrics[0])
	expected := fmt.Sprintf("name:\"test_subsys_meter\" help:\"meter\" type:GAUGE metric:<gauge:<value:%.16f > > ", gm.Rate1())
	if serialized != expected {
		t.Fatalf("Go-metrics value and prometheus metrics value do not match")
	}
}

func TestPrometheusHistogramGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	// values := make([]int64, 0)
	//sample := metrics.HistogramSnapshot{metrics.NewSampleSnapshot(int64(len(values)), values)}
	gm := metrics.NewHistogram(metrics.NewUniformSample(1028))
	metricsRegistry.Register("histogram", gm)

	for ii := 0; ii < 94; ii++ {
		gm.Update(1)
	}
	for ii := 0; ii < 5; ii++ {
		gm.Update(5)
	}
	gm.Update(10)

	go pClient.UpdatePrometheusMetrics()
	time.Sleep(5 * time.Second)
	metrics, _ := prometheusRegistry.Gather()
	if len(metrics) == 0 {
		t.Fatalf("prometheus was unable to register the metric")
	}

	serialized := fmt.Sprint(metrics[0])
	
	serialized = fmt.Sprint(metrics[1])
	expected := "name:\"test_subsys_histogram_max\" help:\"histogram_max\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, float64(gm.Max())) {
		t.Fatalf("Go-metrics value and prometheus metrics value for max do not match:\n+ %s\n- %s", serialized, expected)
	}

	serialized = fmt.Sprint(metrics[2])
	expected = "name:\"test_subsys_histogram_mean\" help:\"histogram_mean\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, gm.Mean()) {
		t.Fatalf("Go-metrics value and prometheus metrics value for mean do not match:\n+ %s\n- %s", serialized, expected)
	}

	serialized = fmt.Sprint(metrics[3])
	expected = "name:\"test_subsys_histogram_min\" help:\"histogram_min\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, float64(gm.Min())) {
		t.Fatalf("Go-metrics value and prometheus metrics value for min do not match:\n+ %s\n- %s", serialized, expected)
	}

	serialized = fmt.Sprint(metrics[4])
	expected = "name:\"test_subsys_histogram_p95\" help:\"histogram_p95\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, gm.Percentile(0.95)) {
		t.Fatalf("Go-metrics value and prometheus metrics value for p95 do not match:\n+ %s\n- %s", serialized, expected)
	}

	serialized = fmt.Sprint(metrics[5])
	expected = "name:\"test_subsys_histogram_p99\" help:\"histogram_p99\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, gm.Percentile(0.99)) {
		t.Fatalf("Go-metrics value and prometheus metrics value for p99 do not match:\n+ %s\n- %s", serialized, expected)
	}

	serialized = fmt.Sprint(metrics[6])
	expected = "name:\"test_subsys_histogram_p999\" help:\"histogram_p999\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, gm.Percentile(0.999)) {
		t.Fatalf("Go-metrics value and prometheus metrics value for p99 do not match:\n+ %s\n- %s", serialized, expected)
	}
}

func TestPrometheusTimerGetUpdated(t *testing.T) {
	prometheusRegistry := prometheus.NewRegistry()
	metricsRegistry := metrics.NewRegistry()
	pClient := NewPrometheusProvider(metricsRegistry, "test", "subsys", prometheusRegistry, 1*time.Second)
	gm := metrics.NewTimer()
	metricsRegistry.Register("timer", gm)

	for ii := 0; ii < 94; ii++ {
		gm.Time(func() {time.Sleep(time.Millisecond)})
	}
	for ii := 0; ii < 5; ii++ {
		gm.Time(func() {time.Sleep(time.Millisecond * 5)})
	}
	gm.Time(func() {time.Sleep(time.Millisecond * 10)})

	go pClient.UpdatePrometheusMetrics()
	time.Sleep(5 * time.Second)
	metrics, _ := prometheusRegistry.Gather()
	if len(metrics) == 0 {
		t.Fatalf("prometheus was unable to register the metric")
	}

	serialized := fmt.Sprint(metrics[0])
	expected := "name:\"test_subsys_timer\" help:\"timer\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, gm.Rate1()) {
		t.Fatalf("Go-metrics value and prometheus metrics value for rate1 do not match:\n+ %s\n- %s", serialized, expected)
	}

	serialized = fmt.Sprint(metrics[1])
	expected = "name:\"test_subsys_timer_max\" help:\"timer_max\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, float64(gm.Max())) {
		t.Fatalf("Go-metrics value and prometheus metrics value for max do not match:\n+ %s\n- %s", serialized, expected)
	}

	serialized = fmt.Sprint(metrics[2])
	expected = "name:\"test_subsys_timer_mean\" help:\"timer_mean\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, gm.Mean()) {
		t.Fatalf("Go-metrics value and prometheus metrics value for mean do not match:\n+ %s\n- %s", serialized, expected)
	}

	serialized = fmt.Sprint(metrics[3])
	expected = "name:\"test_subsys_timer_min\" help:\"timer_min\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, float64(gm.Min())) {
		t.Fatalf("Go-metrics value and prometheus metrics value for min do not match:\n+ %s\n- %s", serialized, expected)
	}

	serialized = fmt.Sprint(metrics[4])
	expected = "name:\"test_subsys_timer_p95\" help:\"timer_p95\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, gm.Percentile(0.95)) {
		t.Fatalf("Go-metrics value and prometheus metrics value for p95 do not match:\n+ %s\n- %s", serialized, expected)
	}

	serialized = fmt.Sprint(metrics[5])
	expected = "name:\"test_subsys_timer_p99\" help:\"timer_p99\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, gm.Percentile(0.99)) {
		t.Fatalf("Go-metrics value and prometheus metrics value for p99 do not match:\n+ %s\n- %s", serialized, expected)
	}

	serialized = fmt.Sprint(metrics[6])
	expected = "name:\"test_subsys_timer_p999\" help:\"timer_p999\" type:GAUGE metric:<gauge:<value"
	if checkMetricText(serialized, expected, gm.Percentile(0.999)) {
		t.Fatalf("Go-metrics value and prometheus metrics value for p99 do not match:\n+ %s\n- %s", serialized, expected)
	}
}

func checkMetricText(serialized, expectedText string, expectedValue float64) bool {
	s := strings.Split(serialized, "value:")
	s2 := strings.Split(s[1], ">")
	serializedValue, _ := strconv.ParseFloat(strings.TrimSpace(s2[0]), 64)

	return numberInRange(serializedValue, expectedValue - 1, expectedValue + 1) && s[0] == expectedText
}

func numberInRange(n, min, max float64) bool {
	return n >= min && n <= max
}